package verify

import (
	"context"
	"testing"
)

func TestExtractHost(t *testing.T) {
	cases := map[string]string{
		"https://api.example.com/v1/users": "api.example.com",
		"http://localhost:8080":            "localhost",
		"example.com":                      "example.com",
		"192.168.1.5:9090":                 "192.168.1.5",
	}
	for input, want := range cases {
		got, err := ExtractHost(input)
		if err != nil {
			t.Fatalf("ExtractHost(%q) unexpected error: %v", input, err)
		}
		if got != want {
			t.Errorf("ExtractHost(%q) = %q, want %q", input, got, want)
		}
	}

	if _, err := ExtractHost(""); err == nil {
		t.Error("expected error for empty target")
	}
}

func TestIsPrivateOrLocalTarget(t *testing.T) {
	private := []string{"localhost", "127.0.0.1", "10.0.0.5", "192.168.1.1", "app.local", "svc.internal"}
	for _, h := range private {
		if !IsPrivateOrLocalTarget(h) {
			t.Errorf("expected %q to be classified as private/local", h)
		}
	}

	if IsPrivateOrLocalTarget("8.8.8.8") {
		t.Error("public IP 8.8.8.8 should not be classified as private")
	}
}

func TestIsBlockedTarget(t *testing.T) {
	blocked := []string{"google.com", "www.google.com", "api.instagram.com", "GITHUB.com"}
	for _, h := range blocked {
		if !IsBlockedTarget(h) {
			t.Errorf("expected %q to be blocked", h)
		}
	}

	if IsBlockedTarget("my-startup-shop.co.id") {
		t.Error("arbitrary customer domain should not be blocked")
	}
}

func TestGenerateChallengeShape(t *testing.T) {
	ch, err := GenerateChallenge("my-startup-shop.co.id")
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}
	if ch.Token == "" {
		t.Error("expected a non-empty token")
	}
	if ch.DNSRecordName != "_oshimai-verify.my-startup-shop.co.id" {
		t.Errorf("unexpected DNS record name: %s", ch.DNSRecordName)
	}
	if ch.WellKnownPath != "/.well-known/oshimai-verify.txt" {
		t.Errorf("unexpected well-known path: %s", ch.WellKnownPath)
	}
	if ch.WellKnownContent != ch.Token {
		t.Error("well-known content should equal the raw token")
	}
}

func TestVerifyChallengeFailsWithoutPublishedProof(t *testing.T) {
	// A domain that (almost certainly) never published our TXT/well-known challenge must fail.
	ch, err := GenerateChallenge("example.com")
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}
	if _, err := VerifyChallenge(context.Background(), "example.com", ch.Token); err == nil {
		t.Error("expected verification to fail for a domain that never published the challenge")
	}
}
