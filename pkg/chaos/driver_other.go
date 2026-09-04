//go:build !linux

package chaos

import (
	"fmt"
	"runtime"
)

// newNetemOrError backs chaos.NewDriverByMode("netem") on non-Linux builds, where tc/netem simply
// doesn't exist — operators on macOS/Windows should use "http_proxy" instead.
func newNetemOrError() (ChaosDriver, error) {
	return nil, fmt.Errorf("netem driver requires Linux + NET_ADMIN (running on %s) — use \"http_proxy\" or \"mock\" instead", runtime.GOOS)
}
