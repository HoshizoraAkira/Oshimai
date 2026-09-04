package k8schaos

import (
	"context"
	"fmt"
	"time"
)

// ReplicaSample is one point-in-time reading of an HPA/Deployment's replica count.
type ReplicaSample struct {
	Time            time.Time `json:"time"`
	CurrentReplicas int       `json:"current_replicas"`
	DesiredReplicas int       `json:"desired_replicas"`
}

// AutoscalerReport is the closed-loop verdict: not just "the app got slow", but "and here is
// whether your autoscaler actually reacted, and how long it took" — the "HPA kamu telat nambah
// pod 40 detik" feature.
type AutoscalerReport struct {
	Namespace          string          `json:"namespace"`
	HPAName            string          `json:"hpa_name"`
	Samples            []ReplicaSample `json:"samples"`
	StartReplicas      int             `json:"start_replicas"`
	PeakReplicas       int             `json:"peak_replicas"`
	ScaledUp           bool            `json:"scaled_up"`
	TimeToFirstScaleUp time.Duration   `json:"time_to_first_scale_up,omitempty"`
	Verdict            string          `json:"verdict"`
}

// MonitorAutoscaler polls an HPA's status every pollInterval until ctx is cancelled (the caller
// ties ctx's lifetime to the load test's duration), recording a full replica-count timeline.
func MonitorAutoscaler(ctx context.Context, client *Client, namespace, hpaName string, pollInterval time.Duration) (*AutoscalerReport, error) {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	report := &AutoscalerReport{Namespace: namespace, HPAName: hpaName}
	startTime := time.Now()
	first := true

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		status, err := client.GetHPAStatus(ctx, namespace, hpaName)
		if err == nil {
			sample := ReplicaSample{Time: time.Now(), CurrentReplicas: status.CurrentReplicas, DesiredReplicas: status.DesiredReplicas}
			report.Samples = append(report.Samples, sample)

			if first {
				report.StartReplicas = status.CurrentReplicas
				first = false
			}
			if status.CurrentReplicas > report.PeakReplicas {
				report.PeakReplicas = status.CurrentReplicas
			}
			if !report.ScaledUp && status.CurrentReplicas > report.StartReplicas {
				report.ScaledUp = true
				report.TimeToFirstScaleUp = time.Since(startTime)
			}
		}

		select {
		case <-ctx.Done():
			report.Verdict = buildAutoscalerVerdict(report)
			if len(report.Samples) == 0 {
				return report, fmt.Errorf("never successfully read HPA %s/%s status", namespace, hpaName)
			}
			return report, nil
		case <-ticker.C:
		}
	}
}

func buildAutoscalerVerdict(r *AutoscalerReport) string {
	if !r.ScaledUp {
		return fmt.Sprintf("HPA %s tidak pernah menambah replica dari baseline %d selama pengujian — autoscaler mungkin tidak aktif atau ambang batasnya terlalu tinggi.", r.HPAName, r.StartReplicas)
	}
	return fmt.Sprintf("HPA %s bereaksi %v setelah beban dimulai, naik dari %d ke puncak %d replica.", r.HPAName, r.TimeToFirstScaleUp.Round(time.Second), r.StartReplicas, r.PeakReplicas)
}
