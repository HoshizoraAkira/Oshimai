package generator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAutoDiscoverViaSitemap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User-agent: *\nSitemap: " + testServerURL(r) + "/sitemap.xml\n"))
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		base := testServerURL(r)
		w.Write([]byte(`<?xml version="1.0"?><urlset><url><loc>` + base + `/</loc></url><url><loc>` + base + `/products</loc></url><url><loc>` + base + `/about</loc></url></urlset>`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>home</body></html>"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	sc, err := AutoDiscover(context.Background(), srv.URL, GeneratorConfig{})
	if err != nil {
		t.Fatalf("AutoDiscover failed: %v", err)
	}
	if len(sc.Steps) < 2 {
		t.Fatalf("expected at least 2 discovered pages from sitemap, got %d", len(sc.Steps))
	}
	if sc.BaseURL != srv.URL {
		t.Errorf("expected base URL %s, got %s", srv.URL, sc.BaseURL)
	}
}

func TestAutoDiscoverFallsBackToHomepageLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><a href="/pricing">Pricing</a><a href="/blog">Blog</a></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	sc, err := AutoDiscover(context.Background(), srv.URL, GeneratorConfig{})
	if err != nil {
		t.Fatalf("AutoDiscover failed: %v", err)
	}
	if len(sc.Steps) < 2 {
		t.Fatalf("expected homepage link scrape to discover at least 2 pages, got %d", len(sc.Steps))
	}
}

func TestAutoDiscoverRejectsInvalidURL(t *testing.T) {
	if _, err := AutoDiscover(context.Background(), "not a url", GeneratorConfig{}); err == nil {
		t.Error("expected error for invalid target URL")
	}
}

func testServerURL(r *http.Request) string {
	scheme := "http"
	return scheme + "://" + r.Host
}
