package chaos

import (
	"runtime"
	"testing"
)

func TestNewDriverByModeMock(t *testing.T) {
	driver, err := NewDriverByMode("mock")
	if err != nil {
		t.Fatalf("mock driver should always be available, got error: %v", err)
	}
	if _, ok := driver.(*MockChaosDriver); !ok {
		t.Errorf("expected *MockChaosDriver, got %T", driver)
	}

	// Default (empty string) should also resolve to mock.
	defaultDriver, err := NewDriverByMode("")
	if err != nil {
		t.Fatalf("default driver mode should resolve to mock, got error: %v", err)
	}
	if _, ok := defaultDriver.(*MockChaosDriver); !ok {
		t.Errorf("expected default mode to be *MockChaosDriver, got %T", defaultDriver)
	}
}

func TestNewDriverByModeHTTPProxy(t *testing.T) {
	driver, err := NewDriverByMode("http_proxy")
	if err != nil {
		t.Fatalf("http_proxy driver should work on every platform, got error: %v", err)
	}
	proxyDriver, ok := driver.(*HTTPProxyDriver)
	if !ok {
		t.Fatalf("expected *HTTPProxyDriver, got %T", driver)
	}
	defer proxyDriver.Close()
}

func TestNewDriverByModeNetem(t *testing.T) {
	driver, err := NewDriverByMode("netem")
	if runtime.GOOS == "linux" {
		if err != nil {
			t.Errorf("expected netem driver to construct on Linux, got error: %v", err)
		}
	} else {
		if err == nil {
			t.Errorf("expected netem driver to be rejected on %s, got driver %T", runtime.GOOS, driver)
		}
	}
}

func TestNewDriverByModeResourceStress(t *testing.T) {
	driver, err := NewDriverByMode("resource_stress")
	if err != nil {
		t.Fatalf("resource_stress driver should work on every platform, got error: %v", err)
	}
	if _, ok := driver.(*ResourceStressDriver); !ok {
		t.Errorf("expected *ResourceStressDriver, got %T", driver)
	}
}

func TestNewDriverByModeUnknown(t *testing.T) {
	if _, err := NewDriverByMode("quantum_tunneling"); err == nil {
		t.Error("expected error for unknown chaos driver mode")
	}
}
