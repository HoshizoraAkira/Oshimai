package loadengine

import (
	"sync"
	"time"
)

type windowBucket struct {
	timestamp time.Time
	requests  int64
	errors    int64
	histogram *HDRHistogram
}

// SlidingWindow provides rolling-window performance metrics for dynamic circuit breaker evaluation.
type SlidingWindow struct {
	mu           sync.RWMutex
	bucketWidth  time.Duration
	windowLength time.Duration
	numBuckets   int
	buckets      []windowBucket
}

// NewSlidingWindow initializes a sliding window with bucket width (typically 1s) and total window length (e.g. 5s).
func NewSlidingWindow(bucketWidth, windowLength time.Duration) *SlidingWindow {
	if bucketWidth <= 0 {
		bucketWidth = 1 * time.Second
	}
	if windowLength < bucketWidth {
		windowLength = bucketWidth
	}
	numBuckets := int(windowLength / bucketWidth)
	if numBuckets < 1 {
		numBuckets = 1
	}

	sw := &SlidingWindow{
		bucketWidth:  bucketWidth,
		windowLength: windowLength,
		numBuckets:   numBuckets,
		buckets:      make([]windowBucket, numBuckets),
	}

	for i := range sw.buckets {
		sw.buckets[i].histogram = NewHDRHistogram()
	}

	return sw
}

// Record registers a single measurement into the active sliding window bucket.
func (sw *SlidingWindow) Record(statusCode int, latency time.Duration, isError bool) {
	now := time.Now()
	nowTrunc := now.Truncate(sw.bucketWidth)

	sw.mu.Lock()
	defer sw.mu.Unlock()

	idx := int((nowTrunc.UnixNano() / int64(sw.bucketWidth)) % int64(sw.numBuckets))
	b := &sw.buckets[idx]

	if !b.timestamp.Equal(nowTrunc) {
		// New time slice in this bucket; reset
		b.timestamp = nowTrunc
		b.requests = 0
		b.errors = 0
		b.histogram = NewHDRHistogram()
	}

	b.requests++
	// isError is the caller's authoritative verdict (e.g. !StepResult.Success): a pure
	// think-time/delay step legitimately reports statusCode 0 with isError=false since it
	// never makes an HTTP call, so status 0 alone must not force an error classification
	// here — that previously misclassified every successful delay step as a failure and
	// could trip the circuit breaker on a scenario that was never actually failing.
	if isError || statusCode >= 500 {
		b.errors++
	}
	b.histogram.Record(latency)
}

// WindowStats contains aggregated metrics across the sliding window duration.
type WindowStats struct {
	Requests   int64
	Errors     int64
	ErrorRate  float64
	P50Latency time.Duration
	P90Latency time.Duration
	P99Latency time.Duration
}

// Stats returns the aggregated request count, error rate, and combined p50, p90, p99 latencies in the active window.
func (sw *SlidingWindow) Stats() WindowStats {
	now := time.Now()
	cutoff := now.Add(-sw.windowLength)

	sw.mu.RLock()
	defer sw.mu.RUnlock()

	var totalReqs int64
	var totalErrs int64
	mergedHist := NewHDRHistogram()

	for i := range sw.buckets {
		b := &sw.buckets[i]
		if b.timestamp.After(cutoff) {
			totalReqs += b.requests
			totalErrs += b.errors
			// Merge histogram bucket counts
			for bi := 0; bi < totalBuckets; bi++ {
				c := b.histogram.buckets[bi]
				if c > 0 {
					mergedHist.buckets[bi] += c
					mergedHist.count += c
				}
			}
		}
	}

	var errRate float64
	if totalReqs > 0 {
		errRate = float64(totalErrs) / float64(totalReqs)
	}

	return WindowStats{
		Requests:   totalReqs,
		Errors:     totalErrs,
		ErrorRate:  errRate,
		P50Latency: mergedHist.Percentile(50.0),
		P90Latency: mergedHist.Percentile(90.0),
		P99Latency: mergedHist.Percentile(99.0),
	}
}
