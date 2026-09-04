package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFileContentDetectsMissingHTTPTimeout(t *testing.T) {
	src := `package main

func fetch() {
	client := http.Client{
		Transport: nil,
	}
	_ = client
}
`
	findings := ScanFileContent("main.go", src)
	if !hasRule(findings, "http-client-no-timeout") {
		t.Errorf("expected http-client-no-timeout finding, got %+v", findings)
	}
}

func TestScanFileContentIgnoresHTTPClientWithTimeout(t *testing.T) {
	src := `package main

func fetch() {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	_ = client
}
`
	findings := ScanFileContent("main.go", src)
	if hasRule(findings, "http-client-no-timeout") {
		t.Errorf("did not expect a finding when Timeout is set, got %+v", findings)
	}
}

func TestScanFileContentDetectsUnboundedRetryLoop(t *testing.T) {
	src := `package main

func callWithRetry() {
	for {
		err := doThing()
		if err == nil {
			return
		}
		// retry after a bit
		time.Sleep(time.Second)
	}
}
`
	findings := ScanFileContent("main.go", src)
	if !hasRule(findings, "unbounded-retry-loop") {
		t.Errorf("expected unbounded-retry-loop finding, got %+v", findings)
	}
}

func TestScanFileContentIgnoresBoundedRetryLoop(t *testing.T) {
	src := `package main

func callWithRetry() {
	for {
		err := doThing()
		if err == nil || attempt >= maxAttempts {
			return
		}
	}
}
`
	findings := ScanFileContent("main.go", src)
	if hasRule(findings, "unbounded-retry-loop") {
		t.Errorf("did not expect a finding when a retry bound is present, got %+v", findings)
	}
}

func TestScanFileContentDetectsBackgroundContextInRequest(t *testing.T) {
	src := `package main

func call() {
	req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	_ = req
}
`
	findings := ScanFileContent("main.go", src)
	if !hasRule(findings, "context-background-in-request") {
		t.Errorf("expected context-background-in-request finding, got %+v", findings)
	}
}

func TestScanFileContentDetectsUnboundedDBPool(t *testing.T) {
	src := `package main

func connect() {
	db, _ := sql.Open("postgres", dsn)
	_ = db
}
`
	findings := ScanFileContent("main.go", src)
	if !hasRule(findings, "db-pool-unbounded") {
		t.Errorf("expected db-pool-unbounded finding, got %+v", findings)
	}
}

func TestScanDirectoryWalksRealFiles(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "service.go")
	os.WriteFile(badFile, []byte(`package service

func fetch() {
	client := http.Client{}
	_ = client
}
`), 0644)

	// Skipped directories must not be descended into.
	os.MkdirAll(filepath.Join(dir, "vendor", "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "vendor", "pkg", "ignored.go"), []byte(`package pkg
func x() { client := http.Client{} ; _ = client }
`), 0644)

	findings, err := ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}
	if !hasRule(findings, "http-client-no-timeout") {
		t.Errorf("expected a finding from service.go, got %+v", findings)
	}
	for _, f := range findings {
		if f.File != "" && filepath.Dir(f.File) == filepath.Join("vendor", "pkg") {
			t.Errorf("expected vendor/ to be skipped, but got finding from it: %+v", f)
		}
	}
}

func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}
