package loadengine

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	linearBuckets = 1000 // 0 to 999 microseconds (1us resolution)
	logFactor     = 1.01 // 1% geometric resolution for >= 1ms
	logBuckets    = 1500 // Covers 1ms to > 3000 seconds
	totalBuckets  = linearBuckets + logBuckets
)

var invLnFactor = 1.0 / math.Log(logFactor)

// HDRHistogram provides low-memory, zero-allocation lock-free microsecond latency recording
// and precise quantile calculation without storing individual latency values in heap.
type HDRHistogram struct {
	buckets [totalBuckets]uint64
	count   uint64
	sumMicros uint64
	minMicros uint64
	maxMicros uint64
}

// NewHDRHistogram creates an initialized histogram.
func NewHDRHistogram() *HDRHistogram {
	h := &HDRHistogram{}
	atomic.StoreUint64(&h.minMicros, math.MaxUint64)
	return h
}

// Record atomically logs a duration into the histogram.
// Lock-free, zero-allocation, thread-safe.
func (h *HDRHistogram) Record(d time.Duration) {
	if d < 0 {
		d = 0
	}
	micros := uint64(d.Microseconds())

	// Compute bucket index
	var idx int
	if micros < linearBuckets {
		idx = int(micros)
	} else {
		ratio := float64(micros) / float64(linearBuckets)
		logIdx := int(math.Log(ratio) * invLnFactor)
		if logIdx < 0 {
			logIdx = 0
		}
		idx = linearBuckets + logIdx
		if idx >= totalBuckets {
			idx = totalBuckets - 1
		}
	}

	atomic.AddUint64(&h.buckets[idx], 1)
	atomic.AddUint64(&h.count, 1)
	atomic.AddUint64(&h.sumMicros, micros)

	// Update min
	for {
		curMin := atomic.LoadUint64(&h.minMicros)
		if micros >= curMin {
			break
		}
		if atomic.CompareAndSwapUint64(&h.minMicros, curMin, micros) {
			break
		}
	}

	// Update max
	for {
		curMax := atomic.LoadUint64(&h.maxMicros)
		if micros <= curMax {
			break
		}
		if atomic.CompareAndSwapUint64(&h.maxMicros, curMax, micros) {
			break
		}
	}
}

// Percentile calculates the latency at percentile p (0.0 < p <= 100.0).
func (h *HDRHistogram) Percentile(p float64) time.Duration {
	total := atomic.LoadUint64(&h.count)
	if total == 0 {
		return 0
	}

	if p <= 0 {
		minM := atomic.LoadUint64(&h.minMicros)
		if minM == math.MaxUint64 {
			return 0
		}
		return time.Duration(minM) * time.Microsecond
	}
	if p >= 100.0 {
		return time.Duration(atomic.LoadUint64(&h.maxMicros)) * time.Microsecond
	}

	rank := uint64(math.Ceil(p / 100.0 * float64(total)))
	var cumulative uint64

	for i := 0; i < totalBuckets; i++ {
		bCount := atomic.LoadUint64(&h.buckets[i])
		if bCount == 0 {
			continue
		}
		cumulative += bCount
		if cumulative >= rank {
			return bucketToDuration(i)
		}
	}

	return time.Duration(atomic.LoadUint64(&h.maxMicros)) * time.Microsecond
}

// Stats returns min, mean, p50, p90, p95, p99, and max.
func (h *HDRHistogram) Stats() LatencyStats {
	total := atomic.LoadUint64(&h.count)
	if total == 0 {
		return LatencyStats{}
	}

	sumM := atomic.LoadUint64(&h.sumMicros)
	meanM := sumM / total

	minM := atomic.LoadUint64(&h.minMicros)
	if minM == math.MaxUint64 {
		minM = 0
	}

	return LatencyStats{
		Min:  time.Duration(minM) * time.Microsecond,
		Mean: time.Duration(meanM) * time.Microsecond,
		P50:  h.Percentile(50.0),
		P90:  h.Percentile(90.0),
		P95:  h.Percentile(95.0),
		P99:  h.Percentile(99.0),
		Max:  time.Duration(atomic.LoadUint64(&h.maxMicros)) * time.Microsecond,
	}
}

func bucketToDuration(idx int) time.Duration {
	if idx < linearBuckets {
		return time.Duration(idx) * time.Microsecond
	}
	logIdx := idx - linearBuckets
	micros := float64(linearBuckets) * math.Exp(float64(logIdx)*math.Log(logFactor))
	return time.Duration(micros) * time.Microsecond
}

// MetricsSnapshot holds a point-in-time view of collected performance metrics.
type MetricsSnapshot struct {
	TotalRequests int64
	TotalErrors   int64
	ErrorRate     float64
	P99Latency    time.Duration
	Timestamp     time.Time
}

// stepMetricsTracker tracks request counts and HDR histogram for an individual step.
type stepMetricsTracker struct {
	totalRequests uint64
	totalErrors   uint64
	histogram     *HDRHistogram
	statusMu      sync.RWMutex
	statusCodes   map[int]int64
}

func newStepMetricsTracker() *stepMetricsTracker {
	return &stepMetricsTracker{
		histogram:   NewHDRHistogram(),
		statusCodes: make(map[int]int64),
	}
}

// MetricsAccumulator coordinates concurrent telemetry aggregation across workers.
type MetricsAccumulator struct {
	totalRequests uint64
	totalErrors   uint64
	histogram     *HDRHistogram

	statusMu    sync.RWMutex
	statusCodes map[int]int64

	stepMu sync.RWMutex
	steps  map[string]*stepMetricsTracker
}

// NewMetricsAccumulator initializes the metrics aggregator.
func NewMetricsAccumulator() *MetricsAccumulator {
	return &MetricsAccumulator{
		histogram:   NewHDRHistogram(),
		statusCodes: make(map[int]int64),
		steps:       make(map[string]*stepMetricsTracker),
	}
}

// Record registers an individual HTTP request outcome.
func (m *MetricsAccumulator) Record(statusCode int, latency time.Duration, isError bool) {
	atomic.AddUint64(&m.totalRequests, 1)
	// See the matching comment in RecordStep below: statusCode 0 is not on its own proof of
	// failure — a successful pure think-time/delay step reports it legitimately.
	if isError || statusCode >= 500 {
		atomic.AddUint64(&m.totalErrors, 1)
	}

	m.histogram.Record(latency)

	m.statusMu.Lock()
	m.statusCodes[statusCode]++
	m.statusMu.Unlock()
}

// RecordStep registers a step-specific HTTP execution outcome.
func (m *MetricsAccumulator) RecordStep(stepID string, statusCode int, latency time.Duration, isError bool) {
	if stepID == "" {
		return
	}
	m.stepMu.RLock()
	tracker, exists := m.steps[stepID]
	m.stepMu.RUnlock()

	if !exists {
		m.stepMu.Lock()
		tracker, exists = m.steps[stepID]
		if !exists {
			tracker = newStepMetricsTracker()
			m.steps[stepID] = tracker
		}
		m.stepMu.Unlock()
	}

	atomic.AddUint64(&tracker.totalRequests, 1)
	// isError is the caller's authoritative verdict (StepResult.Success negated). A pure
	// think-time/delay step has no HTTP response and legitimately reports statusCode 0 while
	// still succeeding, so status 0 alone must not be treated as an error — that previously
	// misclassified every successful delay step as a failure in both the step metrics/heatmap
	// and, via the shared recorder, the circuit breaker's sliding window.
	if isError || statusCode >= 500 {
		atomic.AddUint64(&tracker.totalErrors, 1)
	}
	tracker.histogram.Record(latency)

	tracker.statusMu.Lock()
	tracker.statusCodes[statusCode]++
	tracker.statusMu.Unlock()
}

// Snapshot returns point-in-time request/error counts and p99 latency.
func (m *MetricsAccumulator) Snapshot() MetricsSnapshot {
	reqs := int64(atomic.LoadUint64(&m.totalRequests))
	errs := int64(atomic.LoadUint64(&m.totalErrors))

	var errRate float64
	if reqs > 0 {
		errRate = float64(errs) / float64(reqs)
	}

	return MetricsSnapshot{
		TotalRequests: reqs,
		TotalErrors:   errs,
		ErrorRate:     errRate,
		P99Latency:    m.histogram.Percentile(99.0),
		Timestamp:     time.Now(),
	}
}

// Summary compiles final statistics.
func (m *MetricsAccumulator) Summary(totalDuration time.Duration, status TerminationStatus, abortReason string) *ExecutionSummary {
	reqs := int64(atomic.LoadUint64(&m.totalRequests))
	errs := int64(atomic.LoadUint64(&m.totalErrors))

	var actualRPS float64
	if totalDuration > 0 {
		actualRPS = float64(reqs) / totalDuration.Seconds()
	}

	m.statusMu.RLock()
	scCopy := make(map[int]int64, len(m.statusCodes))
	for k, v := range m.statusCodes {
		scCopy[k] = v
	}
	m.statusMu.RUnlock()

	m.stepMu.RLock()
	stepSummaries := make(map[string]StepSummary, len(m.steps))
	for sID, tr := range m.steps {
		sReqs := int64(atomic.LoadUint64(&tr.totalRequests))
		sErrs := int64(atomic.LoadUint64(&tr.totalErrors))
		var sErrRate float64
		if sReqs > 0 {
			sErrRate = float64(sErrs) / float64(sReqs)
		}

		tr.statusMu.RLock()
		sc := make(map[int]int64, len(tr.statusCodes))
		for k, v := range tr.statusCodes {
			sc[k] = v
		}
		tr.statusMu.RUnlock()

		stepSummaries[sID] = StepSummary{
			StepID:        sID,
			TotalRequests: sReqs,
			TotalErrors:   sErrs,
			ErrorRate:     sErrRate,
			Latency:       tr.histogram.Stats(),
			StatusCodes:   sc,
		}
	}
	m.stepMu.RUnlock()

	return &ExecutionSummary{
		TotalRequests:     reqs,
		TotalErrors:       errs,
		ActualRPS:         actualRPS,
		TotalDuration:     totalDuration,
		StatusCodes:       scCopy,
		Latency:           m.histogram.Stats(),
		TerminationStatus: status,
		AbortReason:       abortReason,
		StepMetrics:       stepSummaries,
	}
}
