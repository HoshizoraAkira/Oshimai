package chaos

import (
	"fmt"
	"strings"
	"time"
)

// BuildNetemArgs transforms a FaultSpec into standard Linux tc netem command tokens.
// Example: ["qdisc", "add", "dev", "eth0", "parent", "1:2", "handle", "20:", "netem", "delay", "100ms", "10ms", "loss", "5%"]
func BuildNetemArgs(spec FaultSpec, parentHandle, handle string) ([]string, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	args := []string{
		"qdisc", "add", "dev", spec.Filter.Interface,
	}

	if parentHandle != "" {
		args = append(args, "parent", parentHandle)
	} else {
		args = append(args, "root")
	}

	if handle != "" {
		args = append(args, "handle", handle)
	}

	args = append(args, "netem")
	var faultAdded bool

	// 1. Latency & Jitter
	if spec.Latency > 0 {
		args = append(args, "delay", formatDuration(spec.Latency))
		faultAdded = true

		if spec.Jitter > 0 {
			args = append(args, formatDuration(spec.Jitter))
			if spec.Correlation > 0 {
				args = append(args, fmt.Sprintf("%.1f%%", spec.Correlation*100))
			}
		}
	}

	// 2. Packet Loss
	if spec.LossPercent > 0 {
		args = append(args, "loss", fmt.Sprintf("%.2f%%", spec.LossPercent))
		faultAdded = true
		if spec.Correlation > 0 && spec.Latency == 0 {
			args = append(args, fmt.Sprintf("%.1f%%", spec.Correlation*100))
		}
	}

	// 3. Packet Corruption
	if spec.CorruptionPercent > 0 {
		args = append(args, "corrupt", fmt.Sprintf("%.2f%%", spec.CorruptionPercent))
		faultAdded = true
	}

	// 4. Bandwidth Rate Limiting
	if spec.RateLimitKbps > 0 {
		args = append(args, "rate", fmt.Sprintf("%dkbit", spec.RateLimitKbps))
		faultAdded = true
	}

	if !faultAdded {
		return nil, fmt.Errorf("at least one fault parameter (latency, loss, corruption, rate_limit) must be specified")
	}

	return args, nil
}

func formatDuration(d time.Duration) string {
	if d >= time.Second && d%time.Second == 0 {
		return fmt.Sprintf("%ds", d/time.Second)
	}
	if d%time.Millisecond == 0 {
		return fmt.Sprintf("%dms", d/time.Millisecond)
	}
	s := d.String()
	return strings.TrimSpace(s)
}
