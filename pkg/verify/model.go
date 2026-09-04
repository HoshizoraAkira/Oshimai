// Package verify implements target-ownership verification and a public-domain blocklist so that
// Oshimai's load and chaos engines cannot be pointed at a domain the operator does not control.
// This closes the "booter-as-a-service" abuse vector that most load/chaos tools leave to a ToS
// disclaimer: here it is enforced in code, the same way certificate authorities validate domain
// ownership before issuing a TLS certificate.
package verify

import "time"

// Method identifies which proof-of-ownership mechanism was used.
type Method string

const (
	MethodDNSTXT           Method = "dns_txt"
	MethodWellKnown        Method = "well_known_file"
	MethodPrivateNet       Method = "private_network"   // No proof required: target is not internet-reachable.
	MethodCloudIPOwnership Method = "cloud_ip_ownership" // The resolved IP belongs to a cloud account the operator holds credentials for.
)

// Challenge is the proof-of-ownership task issued for a target host.
type Challenge struct {
	Host string `json:"host"`
	// DNS TXT record challenge, analogous to ACME dns-01 / domain-verification TXT records.
	DNSRecordName  string `json:"dns_record_name"`
	DNSRecordValue string `json:"dns_record_value"`
	// HTTP well-known file challenge, analogous to ACME http-01.
	WellKnownPath    string    `json:"well_known_path"`
	WellKnownContent string    `json:"well_known_content"`
	Token            string    `json:"-"` // Raw token, kept server-side only.
	IssuedAt         time.Time `json:"issued_at"`
}

// Record tracks the verification lifecycle for one target host.
type Record struct {
	Host       string    `json:"host"`
	Token      string    `json:"-"`
	Verified   bool      `json:"verified"`
	Method     Method    `json:"method,omitempty"`
	VerifiedAt time.Time `json:"verified_at,omitempty"`
}
