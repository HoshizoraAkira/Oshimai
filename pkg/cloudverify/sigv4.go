package cloudverify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// signAWSRequestV4 signs req in place with AWS Signature Version 4, following the algorithm at
// https://docs.aws.amazon.com/general/latest/gr/sigv4-signing-process.html. It is implemented
// directly against net/http rather than pulling in the AWS SDK: the SDK's dependency graph is
// enormous for the one or two EC2 calls cloudverify makes, and SigV4 itself is a fully specified,
// widely-implemented HMAC construction — not a place this package is inventing its own crypto.
// Correctness is pinned by TestSignAWSRequestV4MatchesOfficialTestVector against AWS's own
// published worked example.
func signAWSRequestV4(req *http.Request, body []byte, accessKey, secretKey, region, service string, now time.Time) error {
	amzDate := now.UTC().Format("20060102T150405Z")
	dateStamp := now.UTC().Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	if req.Header.Get("Host") == "" {
		req.Header.Set("Host", req.URL.Host)
	}

	canonicalHeaders, signedHeaders := canonicalizeHeaders(req.Header, req.URL.Host)
	payloadHash := sha256Hex(body)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL.Path),
		canonicalQuery(req.URL.RawQuery),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(secretKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	return nil
}

func canonicalURI(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

// canonicalQuery sorts query parameters by key (AWS's canonical query string requirement); Go's
// url.Values.Encode() already sorts by key, so parsing and re-encoding is sufficient.
func canonicalQuery(rawQuery string) string {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	return values.Encode()
}

func canonicalizeHeaders(header http.Header, host string) (canonical, signedList string) {
	// Only Host and X-Amz-Date are signed — sufficient for the query-string EC2 API calls this
	// package makes, which carry no other required signed headers.
	entries := map[string]string{
		"host":       strings.ToLower(host),
		"x-amz-date": header.Get("X-Amz-Date"),
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var canonicalBuilder strings.Builder
	for _, k := range keys {
		canonicalBuilder.WriteString(k)
		canonicalBuilder.WriteByte(':')
		canonicalBuilder.WriteString(strings.TrimSpace(entries[k]))
		canonicalBuilder.WriteByte('\n')
	}
	return canonicalBuilder.String(), strings.Join(keys, ";")
}

func deriveSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
