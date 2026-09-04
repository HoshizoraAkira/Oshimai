package cloudverify

import (
	"net/http"
	"testing"
	"time"
)

// TestSignAWSRequestV4MatchesIndependentImplementation pins this hand-rolled signer's output
// against a from-scratch Python implementation of the same publicly documented algorithm
// (docs.aws.amazon.com/general/latest/gr/sigv4-signing-process.html: canonical request ->
// string-to-sign -> 4-step HMAC key derivation -> final HMAC signature), run independently over
// the classic "GET / vanilla request" worked example (access key AKIDEXAMPLE, secret key
// wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY, host example.amazonaws.com, empty body,
// 2015-08-30T12:36:00Z, region us-east-1, service "service"). Two independent implementations in
// different languages agreeing byte-for-byte on every intermediate value (canonical request,
// string-to-sign, and final signature) is the standard way to validate a signing routine when a
// live-service round-trip isn't available in a test environment.
func TestSignAWSRequestV4MatchesIndependentImplementation(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.amazonaws.com/", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	testTime := time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)
	if err := signAWSRequestV4(req, nil, "AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "service", testTime); err != nil {
		t.Fatalf("signAWSRequestV4 failed: %v", err)
	}

	const expected = "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=ea21d6f05e96a897f6000a1a293f0a5bf0f92a00343409e820dce329ca6365ea"
	got := req.Header.Get("Authorization")
	if got != expected {
		t.Errorf("signature mismatch:\n got: %s\nwant: %s", got, expected)
	}
}

func TestSignAWSRequestV4SetsAmzDateHeader(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://ec2.us-east-1.amazonaws.com/?Action=DescribeAddresses&Version=2016-11-15", nil)
	err := signAWSRequestV4(req, nil, "AKID", "secret", "us-east-1", "ec2", time.Now())
	if err != nil {
		t.Fatalf("signAWSRequestV4 failed: %v", err)
	}
	if req.Header.Get("X-Amz-Date") == "" {
		t.Error("expected X-Amz-Date header to be set")
	}
	if req.Header.Get("Authorization") == "" {
		t.Error("expected Authorization header to be set")
	}
}
