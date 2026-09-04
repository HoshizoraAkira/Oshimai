package chaos

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// hopByHopHeaders lists headers that are connection-scoped and must never be forwarded by a
// proxy, per RFC 7230 §6.1 (the same list net/http/httputil.ReverseProxy strips internally).
var hopByHopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// HTTPProxyDriver implements ChaosDriver as a userspace forward HTTP proxy: instead of
// manipulating the kernel's network stack (tc/netem, which needs Linux + NET_ADMIN/root),
// Virtual Users route their requests through this proxy via the standard http.Transport.Proxy
// mechanism, and fault injection happens entirely in application code. This makes chaos testing
// available on macOS, Windows, locked-down containers, and CI runners — anywhere a Go binary can
// listen on a loopback port. The trade-off: only plain HTTP targets are supported (no CONNECT
// tunneling for HTTPS), since faults are applied to real HTTP semantics, not opaque encrypted
// bytes.
type HTTPProxyDriver struct {
	mu           sync.RWMutex
	status       ChaosStatus
	activeFault  *FaultSpec
	listener     net.Listener
	server       *http.Server
	rng          *rand.Rand
	rngMu        sync.Mutex
	roundTripper http.RoundTripper
}

// NewHTTPProxyDriver starts listening on an ephemeral loopback port and returns a ready-to-use
// driver. The proxy always forwards requests faithfully when idle; Apply/Revert only toggle
// whether faults are currently injected into that forwarding path.
func NewHTTPProxyDriver() (*HTTPProxyDriver, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start HTTP chaos proxy listener: %w", err)
	}

	d := &HTTPProxyDriver{
		status:       ChaosStatus{State: StateIdle},
		listener:     listener,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
		roundTripper: http.DefaultTransport,
	}
	d.server = &http.Server{Handler: d}

	go func() {
		_ = d.server.Serve(listener)
	}()

	return d, nil
}

// ProxyURL returns the local forward-proxy address (e.g. "http://127.0.0.1:54321") to configure
// as an http.Transport's Proxy.
func (d *HTTPProxyDriver) ProxyURL() string {
	return "http://" + d.listener.Addr().String()
}

// HTTPClient returns an *http.Client pre-configured to route all outbound requests through this
// proxy — ready to hand to loadengine.EngineConfig.Client.
func (d *HTTPProxyDriver) HTTPClient() (*http.Client, error) {
	proxyURL, err := url.Parse(d.ProxyURL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse chaos proxy URL: %w", err)
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}, nil
}

// Apply activates fault injection on already-forwarded requests. Unlike LinuxNetemDriver, no
// kernel qdisc is touched — this simply flips a guarded pointer that ServeHTTP consults per request.
func (d *HTTPProxyDriver) Apply(ctx context.Context, fault FaultSpec) error {
	if err := fault.Validate(); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	faultCopy := fault
	d.activeFault = &faultCopy
	now := time.Now()
	d.status = ChaosStatus{
		State:        StateInjected,
		CurrentFault: &faultCopy,
		AppliedAt:    now,
		ExpiresAt:    now.Add(fault.Duration),
	}
	return nil
}

// Revert idempotently disables fault injection; the proxy keeps forwarding traffic normally.
func (d *HTTPProxyDriver) Revert(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeFault = nil
	d.status = ChaosStatus{State: StateReverted}
	return nil
}

// Status returns a copy of the live chaos status.
func (d *HTTPProxyDriver) Status() ChaosStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.status
}

// Close stops the underlying listener. Safe to call once the driver is no longer needed.
func (d *HTTPProxyDriver) Close() error {
	return d.server.Close()
}

func (d *HTTPProxyDriver) currentFault() *FaultSpec {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.activeFault
}

// ServeHTTP implements the forward-proxy handler with fault injection interposed between
// receiving the client's request and relaying the origin's response.
func (d *HTTPProxyDriver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		http.Error(w, "oshimai http_proxy chaos driver supports plain HTTP targets only (no HTTPS/CONNECT tunneling)", http.StatusBadGateway)
		return
	}
	if !r.URL.IsAbs() {
		http.Error(w, "request must use an absolute-URI (this is a forward proxy, not an origin server)", http.StatusBadRequest)
		return
	}

	fault := d.currentFault()
	// A fault with TargetDomains set only applies to matching hosts — everything else (including
	// the control plane's own traffic) passes through untouched. This is what makes "chaos
	// dependency pihak ketiga" possible: target api.midtrans.com without degrading your own app.
	applies := fault != nil && domainMatches(r.URL.Hostname(), fault.Filter.TargetDomains)

	if applies && d.roll(fault.DNSFailPercent) {
		http.Error(w, fmt.Sprintf("oshimai chaos proxy: simulated DNS resolution failure for %s", r.URL.Hostname()), http.StatusBadGateway)
		return
	}
	if applies && fault.DNSDelay > 0 {
		time.Sleep(fault.DNSDelay)
	}

	if applies && d.roll(fault.LossPercent) {
		// Simulate packet loss: sever the connection without any response, the same symptom a
		// client sees from a real dropped packet or a reset connection.
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				conn.Close()
				return
			}
		}
		w.WriteHeader(499)
		return
	}

	if applies {
		time.Sleep(jitteredLatency(fault.Latency, fault.Jitter, d.randFloat))
	}

	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	stripHopByHopHeaders(outReq.Header)

	if applies && fault.ClockSkewOffset != 0 {
		// Real OS clock skew needs root; this is the app-level equivalent: a cooperating target
		// reads this header to simulate its own clock being off by the given offset (useful for
		// testing JWT/session-expiry handling without touching the host clock).
		outReq.Header.Set("X-Oshimai-Simulated-Time", time.Now().Add(fault.ClockSkewOffset).Format(time.RFC3339))
	}

	resp, err := d.roundTripper.RoundTrip(outReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("oshimai chaos proxy: upstream request failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if applies && d.roll(fault.CorruptionPercent) {
		// Simulate response corruption as an application would observe it: a broken/garbled
		// response the client's JSON/body parsing would choke on.
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("oshimai: simulated response corruption fault"))
		return
	}

	stripHopByHopHeaders(resp.Header)
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if applies && fault.RateLimitKbps > 0 {
		throttledCopy(w, resp.Body, fault.RateLimitKbps)
	} else {
		_, _ = io.Copy(w, resp.Body)
	}
}

// domainMatches reports whether host satisfies domains: an empty list matches everything
// (fault applies proxy-wide), otherwise host must equal or be a subdomain of one of the entries.
func domainMatches(host string, domains []string) bool {
	if len(domains) == 0 {
		return true
	}
	host = strings.ToLower(host)
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

func (d *HTTPProxyDriver) roll(percent float64) bool {
	if percent <= 0 {
		return false
	}
	return d.randFloat()*100 < percent
}

func (d *HTTPProxyDriver) randFloat() float64 {
	d.rngMu.Lock()
	defer d.rngMu.Unlock()
	return d.rng.Float64()
}

// jitteredLatency computes a randomized delay: base latency plus uniform jitter in [-jitter, +jitter],
// clamped to zero, mirroring the netem latency/jitter convention used by LinuxNetemDriver.
func jitteredLatency(latency, jitter time.Duration, randFloat func() float64) time.Duration {
	if latency <= 0 {
		return 0
	}
	if jitter <= 0 {
		return latency
	}
	offset := time.Duration((randFloat()*2 - 1) * float64(jitter))
	total := latency + offset
	if total < 0 {
		return 0
	}
	return total
}

const throttleChunkBytes = 2048

// throttledCopy relays src to dst in small chunks, sleeping between them to approximate
// rateKbps — a simple software token-bucket that needs no extra dependency.
func throttledCopy(dst io.Writer, src io.Reader, rateKbps int) {
	if rateKbps <= 0 {
		_, _ = io.Copy(dst, src)
		return
	}
	bytesPerSecond := float64(rateKbps) * 1024 / 8
	chunkDuration := time.Duration(float64(throttleChunkBytes) / bytesPerSecond * float64(time.Second))

	buf := make([]byte, throttleChunkBytes)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
			if flusher, ok := dst.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(chunkDuration)
		}
		if err != nil {
			return
		}
	}
}

func stripHopByHopHeaders(h http.Header) {
	for _, header := range hopByHopHeaders {
		h.Del(header)
	}
}
