package cloudverify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAWSCheckerOwnsIPFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected a signed request with an Authorization header")
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<?xml version="1.0"?>
<DescribeAddressesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <addressesSet>
    <item>
      <publicIp>203.0.113.5</publicIp>
      <instanceId>i-0abcdef1234567890</instanceId>
    </item>
  </addressesSet>
</DescribeAddressesResponse>`))
	}))
	defer srv.Close()

	checker := NewAWSChecker("AKIDEXAMPLE", "secret", "us-east-1")
	checker.endpoint = srv.URL + "/"

	owns, resource, err := checker.OwnsIP(context.Background(), "203.0.113.5")
	if err != nil {
		t.Fatalf("OwnsIP failed: %v", err)
	}
	if !owns {
		t.Fatal("expected the IP to be reported as owned")
	}
	if !strings.Contains(resource, "i-0abcdef1234567890") {
		t.Errorf("expected the resource description to mention the instance ID, got %q", resource)
	}
}

func TestAWSCheckerOwnsIPNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<DescribeAddressesResponse><addressesSet></addressesSet></DescribeAddressesResponse>`))
	}))
	defer srv.Close()

	checker := NewAWSChecker("AKID", "secret", "us-east-1")
	checker.endpoint = srv.URL + "/"

	owns, _, err := checker.OwnsIP(context.Background(), "198.51.100.9")
	if err != nil {
		t.Fatalf("OwnsIP failed: %v", err)
	}
	if owns {
		t.Error("expected the IP to be reported as not owned when absent from the response")
	}
}

func TestAWSCheckerOwnsIPPropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<Response><Errors><Error><Code>AuthFailure</Code><Message>bad credentials</Message></Error></Errors></Response>`))
	}))
	defer srv.Close()

	checker := NewAWSChecker("AKID", "wrong-secret", "us-east-1")
	checker.endpoint = srv.URL + "/"

	if _, _, err := checker.OwnsIP(context.Background(), "203.0.113.5"); err == nil {
		t.Error("expected an error to propagate from a failed EC2 API call")
	}
}
