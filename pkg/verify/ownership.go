package verify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// blockedApexDomains lists high-traffic public domains that must never be accepted as a load or
// chaos test target, regardless of ownership proof (nobody can legitimately "prove" ownership of
// google.com from inside Oshimai, so these are rejected outright rather than routed through the
// challenge flow).
var blockedApexDomains = map[string]bool{
	"google.com": true, "googleapis.com": true, "gmail.com": true,
	"facebook.com": true, "instagram.com": true, "whatsapp.com": true, "meta.com": true,
	"amazon.com": true, "amazonaws.com": true, "aws.amazon.com": true,
	"microsoft.com": true, "azure.com": true, "office.com": true, "live.com": true,
	"apple.com": true, "icloud.com": true,
	"cloudflare.com": true, "akamai.com": true, "fastly.com": true,
	"github.com": true, "gitlab.com": true, "npmjs.com": true,
	"twitter.com": true, "x.com": true, "tiktok.com": true, "youtube.com": true,
	"wikipedia.org": true, "yahoo.com": true, "bing.com": true,
	"tokopedia.com": true, "shopee.co.id": true, "bukalapak.com": true, "blibli.com": true,
	"gojek.com": true, "grab.com": true, "bca.co.id": true, "bri.co.id": true, "mandiri.co.id": true,
	"telkomsel.com": true, "indihome.co.id": true, "kominfo.go.id": true, "go.id": true,
}

// ExtractHost normalizes a user-supplied target URL (or bare host) down to a lowercase hostname.
func ExtractHost(rawTarget string) (string, error) {
	rawTarget = strings.TrimSpace(rawTarget)
	if rawTarget == "" {
		return "", fmt.Errorf("target cannot be empty")
	}
	candidate := rawTarget
	if !strings.Contains(candidate, "://") {
		candidate = "http://" + candidate
	}
	u, err := url.Parse(candidate)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("could not parse target host from %q", rawTarget)
	}
	return strings.ToLower(u.Hostname()), nil
}

// IsPrivateOrLocalTarget reports whether host resolves only to loopback/private/link-local
// addresses (or is a bare "localhost"-style name). These never require ownership verification —
// they are not reachable from the public internet, so there is nothing to protect a third party
// from.
func IsPrivateOrLocalTarget(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return true
	}
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") || strings.HasSuffix(h, ".test") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}
	// Best-effort resolution: if every resolved address is private/loopback, treat as private.
	addrs, err := net.LookupHost(h)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil || !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
			return false
		}
	}
	return true
}

// IsBlockedTarget reports whether host (or one of its parent domains) is on the public blocklist.
func IsBlockedTarget(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	labels := strings.Split(h, ".")
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		if blockedApexDomains[candidate] {
			return true
		}
	}
	return blockedApexDomains[h]
}

// GenerateChallenge issues a fresh proof-of-ownership challenge for host.
func GenerateChallenge(host string) (Challenge, error) {
	var token string
	recordName := fmt.Sprintf("_oshimai-verify.%s", host)
	if txts, err := net.DefaultResolver.LookupTXT(context.Background(), recordName); err == nil {
		for _, t := range txts {
			t = strings.TrimSpace(t)
			if strings.HasPrefix(t, "oshimai-verify=") {
				existing := strings.TrimPrefix(t, "oshimai-verify=")
				if len(existing) == 32 {
					token = existing
					break
				}
			}
		}
	}

	if token == "" {
		tokenBytes := make([]byte, 16)
		if _, err := rand.Read(tokenBytes); err != nil {
			return Challenge{}, fmt.Errorf("failed to generate verification token: %w", err)
		}
		token = hex.EncodeToString(tokenBytes)
	}

	return Challenge{
		Host:             host,
		DNSRecordName:    fmt.Sprintf("_oshimai-verify.%s", host),
		DNSRecordValue:   fmt.Sprintf("oshimai-verify=%s", token),
		WellKnownPath:    "/.well-known/oshimai-verify.txt",
		WellKnownContent: token,
		Token:            token,
		IssuedAt:         time.Now(),
	}, nil
}

// VerifyChallenge checks whether host has published token via DNS TXT or the well-known HTTP file.
// It returns the method that succeeded, or an error explaining why neither check passed.
func VerifyChallenge(ctx context.Context, host, token string) (Method, error) {
	if method, ok := verifyDNS(ctx, host, token); ok {
		return method, nil
	}
	if method, ok := verifyWellKnown(ctx, host, token); ok {
		return method, nil
	}
	return "", fmt.Errorf("no matching DNS TXT record at _oshimai-verify.%s and no /.well-known/oshimai-verify.txt found on %s", host, host)
}

func verifyDNS(ctx context.Context, host, token string) (Method, bool) {
	recordName := fmt.Sprintf("_oshimai-verify.%s", host)
	txts, err := net.DefaultResolver.LookupTXT(ctx, recordName)
	if err != nil {
		return "", false
	}
	expected := fmt.Sprintf("oshimai-verify=%s", token)
	for _, t := range txts {
		if strings.TrimSpace(t) == expected {
			return MethodDNSTXT, true
		}
	}
	return "", false
}

func verifyWellKnown(ctx context.Context, host, token string) (Method, bool) {
	client := &http.Client{Timeout: 8 * time.Second}
	for _, scheme := range []string{"https", "http"} {
		reqURL := fmt.Sprintf("%s://%s/.well-known/oshimai-verify.txt", scheme, host)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		if strings.TrimSpace(string(body)) == token {
			return MethodWellKnown, true
		}
	}
	return "", false
}
