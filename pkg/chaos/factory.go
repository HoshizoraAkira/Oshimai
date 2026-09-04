package chaos

import "fmt"

// NewDriverByMode constructs a ChaosDriver by name, so the deployment (CLI flag, config file)
// decides how fault injection actually happens without the caller needing platform-specific
// build tags:
//
//   - "mock":       in-memory, zero side effects — safe default for local dev and CI.
//   - "http_proxy": userspace forward-proxy fault injection (pkg/chaos.HTTPProxyDriver) — works
//     on any OS, any container, without root or NET_ADMIN. HTTP targets only.
//   - "netem":      Linux tc/netem kernel-level fault injection — most realistic, but requires
//     Linux + root/NET_ADMIN. Returns an error on any other platform.
//   - "resource_stress": CPU/memory/disk pressure on the HOST this driver runs on (see
//     ResourceStressDriver) — for testing an app that shares a host with Oshimai/an agent.
func NewDriverByMode(mode string) (ChaosDriver, error) {
	switch mode {
	case "", "mock":
		return NewMockChaosDriver(), nil
	case "http_proxy":
		return NewHTTPProxyDriver()
	case "netem":
		return newNetemOrError()
	case "resource_stress":
		return NewResourceStressDriver(), nil
	default:
		return nil, fmt.Errorf("unknown chaos driver mode %q (want mock|http_proxy|netem|resource_stress)", mode)
	}
}
