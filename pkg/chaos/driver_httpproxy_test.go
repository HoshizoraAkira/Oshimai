package chaos

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func newTestProxyDriver(t *testing.T) (*HTTPProxyDriver, *http.Client) {
	t.Helper()
	driver, err := NewHTTPProxyDriver()
	if err != nil {
		t.Fatalf("NewHTTPProxyDriver failed: %v", err)
	}
	t.Cleanup(func() { driver.Close() })

	client, err := driver.HTTPClient()
	if err != nil {
		t.Fatalf("HTTPClient failed: %v", err)
	}
	return driver, client
}

func TestHTTPProxyDriverPassthroughWhenIdle(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))
	defer origin.Close()

	_, client := newTestProxyDriver(t)

	resp, err := client.Get(origin.URL + "/ping")
	if err != nil {
		t.Fatalf("request through idle proxy failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("expected passthrough body %q, got %q", "hello", body)
	}
}

func TestHTTPProxyDriverInjectsLatency(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	driver, client := newTestProxyDriver(t)
	err := driver.Apply(context.Background(), FaultSpec{
		ID:       "latency-test",
		Type:     FaultLatency,
		Filter:   FilterConfig{Interface: "lo"},
		Latency:  150 * time.Millisecond,
		Duration: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	start := time.Now()
	resp, err := client.Get(origin.URL + "/slow")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if elapsed < 150*time.Millisecond {
		t.Errorf("expected injected latency of at least 150ms, request took only %v", elapsed)
	}
}

func TestHTTPProxyDriverInjectsPacketLoss(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	driver, client := newTestProxyDriver(t)
	client.Timeout = 2 * time.Second
	err := driver.Apply(context.Background(), FaultSpec{
		ID:          "loss-test",
		Type:        FaultPacketLoss,
		Filter:      FilterConfig{Interface: "lo"},
		LossPercent: 100, // Deterministic: every request must be dropped.
		Duration:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	_, err = client.Get(origin.URL + "/dropped")
	if err == nil {
		t.Error("expected request to fail when packet loss is 100%")
	}
}

func TestHTTPProxyDriverInjectsCorruption(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer origin.Close()

	driver, client := newTestProxyDriver(t)
	err := driver.Apply(context.Background(), FaultSpec{
		ID:                "corrupt-test",
		Type:              FaultCorruption,
		Filter:            FilterConfig{Interface: "lo"},
		CorruptionPercent: 100, // Deterministic corruption.
		Duration:          5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	resp, err := client.Get(origin.URL + "/data")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected simulated corruption to surface as 500, got %d", resp.StatusCode)
	}
}

func TestHTTPProxyDriverRevertRestoresPassthrough(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	driver, client := newTestProxyDriver(t)
	client.Timeout = 2 * time.Second
	_ = driver.Apply(context.Background(), FaultSpec{
		ID: "revert-test", Type: FaultPacketLoss, Filter: FilterConfig{Interface: "lo"},
		LossPercent: 100, Duration: 5 * time.Second,
	})

	if err := driver.Revert(context.Background()); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}

	resp, err := client.Get(origin.URL + "/after-revert")
	if err != nil {
		t.Fatalf("expected request to succeed after revert, got error: %v", err)
	}
	resp.Body.Close()

	if status := driver.Status(); status.State != StateReverted {
		t.Errorf("expected state reverted, got %s", status.State)
	}
}

func TestHTTPProxyDriverRejectsCONNECT(t *testing.T) {
	driver, err := NewHTTPProxyDriver()
	if err != nil {
		t.Fatalf("NewHTTPProxyDriver failed: %v", err)
	}
	defer driver.Close()

	// Send a literal CONNECT request over a raw TCP connection — this is how a real
	// http.Transport asks a forward proxy to tunnel an HTTPS target, and isn't something
	// net/http's client API can construct directly via client.Do.
	conn, err := net.Dial("tcp", driver.listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to dial proxy: %v", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("failed to read proxy response: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 for unsupported CONNECT, got %d", resp.StatusCode)
	}
}

func TestHTTPProxyDriverDomainScopedFault(t *testing.T) {
	var sawSkewHeader string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawSkewHeader = r.Header.Get("X-Oshimai-Simulated-Time")
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	driver, client := newTestProxyDriver(t)
	client.Timeout = 2 * time.Second

	// Scope the fault to a domain that does NOT match the test origin — loss must NOT apply.
	err := driver.Apply(context.Background(), FaultSpec{
		ID: "scoped-test", Type: FaultPacketLoss, Filter: FilterConfig{Interface: "lo", TargetDomains: []string{"api.midtrans.com"}},
		LossPercent: 100, Duration: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if _, err := client.Get(origin.URL + "/unaffected"); err != nil {
		t.Errorf("expected request to an out-of-scope domain to pass through, got error: %v", err)
	}

	// Now scope it to the actual origin host — the fault must apply.
	originURL, _ := url.Parse(origin.URL)
	host := originURL.Hostname()
	_ = driver.Apply(context.Background(), FaultSpec{
		ID: "scoped-test-2", Type: FaultClockSkew, Filter: FilterConfig{Interface: "lo", TargetDomains: []string{host}},
		ClockSkewOffset: -48 * time.Hour, Duration: 5 * time.Second,
	})
	resp, err := client.Get(origin.URL + "/affected")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if sawSkewHeader == "" {
		t.Error("expected X-Oshimai-Simulated-Time header to be injected for the in-scope domain")
	}
}

func TestHTTPProxyDriverDNSFault(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	driver, client := newTestProxyDriver(t)
	client.Timeout = 2 * time.Second
	err := driver.Apply(context.Background(), FaultSpec{
		ID: "dns-fail-test", Type: FaultDNS, Filter: FilterConfig{Interface: "lo"},
		DNSFailPercent: 100, Duration: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	resp, err := client.Get(origin.URL + "/x")
	if err != nil {
		t.Fatalf("request-level error (unexpected, proxy should return a 502 body): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 for simulated DNS failure, got %d", resp.StatusCode)
	}
}

func TestDomainMatches(t *testing.T) {
	cases := []struct {
		host    string
		domains []string
		want    bool
	}{
		{"api.midtrans.com", nil, true},
		{"api.midtrans.com", []string{"api.midtrans.com"}, true},
		{"sandbox.api.midtrans.com", []string{"midtrans.com"}, true},
		{"api.xendit.co", []string{"midtrans.com"}, false},
	}
	for _, c := range cases {
		if got := domainMatches(c.host, c.domains); got != c.want {
			t.Errorf("domainMatches(%q, %v) = %v, want %v", c.host, c.domains, got, c.want)
		}
	}
}

func TestHTTPProxyDriverRejectsInvalidFault(t *testing.T) {
	driver, err := NewHTTPProxyDriver()
	if err != nil {
		t.Fatalf("NewHTTPProxyDriver failed: %v", err)
	}
	defer driver.Close()

	if err := driver.Apply(context.Background(), FaultSpec{ID: ""}); err == nil {
		t.Error("expected validation error for empty fault spec")
	}
}
