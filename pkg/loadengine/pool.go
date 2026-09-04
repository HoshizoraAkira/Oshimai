package loadengine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

type stepRecorder interface {
	Record(statusCode int, latency time.Duration, isError bool)
	RecordStep(stepID string, statusCode int, latency time.Duration, isError bool)
}

// executeSession runs a single VirtualUser session and pipes telemetry to the metrics accumulators.
func executeSession(ctx context.Context, id string, sc *vusession.Scenario, client vusession.HTTPClient, recorder stepRecorder) error {
	vu, err := vusession.NewVirtualUser(vusession.Config{
		ID:       id,
		Scenario: sc,
		Client:   client,
	})
	if err != nil {
		return err
	}

	report, err := vu.Execute(ctx)
	if report != nil {
		for _, stepRes := range report.StepResults {
			recorder.RecordStep(stepRes.StepID, stepRes.StatusCode, stepRes.Duration, !stepRes.Success)
		}
	}
	return err
}

// runFlatVU orchestrates a fixed pool of concurrent workers continuously executing scenarios.
func runFlatVU(ctx context.Context, cfg EngineConfig, sc *vusession.Scenario, recorder stepRecorder, activeVUs *atomic.Int32) {
	var wg sync.WaitGroup
	wg.Add(cfg.VUs)

	var sessionSeq uint64

	for i := 0; i < cfg.VUs; i++ {
		workerID := i
		if activeVUs != nil {
			activeVUs.Add(1)
		}
		go func() {
			defer func() {
				if activeVUs != nil {
					activeVUs.Add(-1)
				}
				wg.Done()
			}()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				seq := atomic.AddUint64(&sessionSeq, 1)
				vuID := fmt.Sprintf("vu-%d-%d", workerID, seq)
				_ = executeSession(ctx, vuID, sc, cfg.Client, recorder)
			}
		}()
	}

	wg.Wait()
}

// runTargetRPS orchestrates load generation using a precise token bucket rate limiter.
func runTargetRPS(ctx context.Context, cfg EngineConfig, sc *vusession.Scenario, recorder stepRecorder, activeVUs *atomic.Int32) {
	if cfg.TargetRPS <= 0 {
		return
	}

	interval := time.Duration(float64(time.Second) / cfg.TargetRPS)
	if interval < time.Microsecond {
		interval = time.Microsecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Max inflight concurrency limiter to prevent unbounded memory growth if target slows down
	maxInflight := int(cfg.TargetRPS * 10)
	if maxInflight < 200 {
		maxInflight = 200
	} else if maxInflight > 5000 {
		maxInflight = 5000
	}

	sem := make(chan struct{}, maxInflight)
	var wg sync.WaitGroup
	var sessionSeq uint64

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-ticker.C:
			select {
			case sem <- struct{}{}:
				wg.Add(1)
				if activeVUs != nil {
					activeVUs.Add(1)
				}
				seq := atomic.AddUint64(&sessionSeq, 1)
				vuID := fmt.Sprintf("rps-vu-%d", seq)

				go func(id string) {
					defer func() {
						if activeVUs != nil {
							activeVUs.Add(-1)
						}
						<-sem
						wg.Done()
					}()
					_ = executeSession(ctx, id, sc, cfg.Client, recorder)
				}(vuID)
			default:
				// Capacity saturated, record dropped / overload
				recorder.Record(0, 0, true)
			}
		}
	}
}

// runRamping orchestrates dynamic multi-stage scaling (ramp-up, sustain, ramp-down).
func runRamping(ctx context.Context, cfg EngineConfig, sc *vusession.Scenario, recorder stepRecorder, activeVUs *atomic.Int32) {
	var currentVUs int32
	var sessionSeq uint64
	var workerWg sync.WaitGroup

	stopSignal := make(chan struct{})
	defer close(stopSignal)

	startWorker := func(wID int) {
		workerWg.Add(1)
		atomic.AddInt32(&currentVUs, 1)
		if activeVUs != nil {
			activeVUs.Add(1)
		}

		go func() {
			defer func() {
				if activeVUs != nil {
					activeVUs.Add(-1)
				}
				atomic.AddInt32(&currentVUs, -1)
				workerWg.Done()
			}()

			for {
				select {
				case <-ctx.Done():
					return
				case <-stopSignal:
					return
				default:
				}

				// Check if current active exceeds desired target
				desired := atomic.LoadInt32(&currentVUs)
				if desired < 0 {
					return
				}

				seq := atomic.AddUint64(&sessionSeq, 1)
				vuID := fmt.Sprintf("ramp-vu-%d-%d", wID, seq)
				_ = executeSession(ctx, vuID, sc, cfg.Client, recorder)
			}
		}()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	stageStart := time.Now()
	stageIdx := 0
	initialVUs := 0

	totalStages := len(cfg.Stages)
	if totalStages == 0 {
		return
	}

	for stageIdx < totalStages {
		stage := cfg.Stages[stageIdx]
		stageElapsed := time.Since(stageStart)

		if stageElapsed >= stage.Duration {
			// Move to next stage
			initialVUs = stage.TargetVUs
			stageIdx++
			stageStart = time.Now()
			continue
		}

		// Linear interpolation: targetVUs = initial + (target - initial) * (elapsed / duration)
		progress := float64(stageElapsed) / float64(stage.Duration)
		if progress > 1.0 {
			progress = 1.0
		}
		targetVUCount := int(float64(initialVUs) + float64(stage.TargetVUs-initialVUs)*progress)

		cur := int(atomic.LoadInt32(&currentVUs))
		if cur < targetVUCount {
			diff := targetVUCount - cur
			for d := 0; d < diff; d++ {
				startWorker(cur + d)
			}
		}

		select {
		case <-ctx.Done():
			workerWg.Wait()
			return
		case <-ticker.C:
		}
	}

	workerWg.Wait()
}
