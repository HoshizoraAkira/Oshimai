package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/oshimai/twin/pkg/verify"
)

// VerificationStore tracks in-progress and completed target-ownership verifications for the
// control plane's lifetime. It is the stateful counterpart to the pure functions in pkg/verify.
type VerificationStore struct {
	mu         sync.RWMutex
	challenges map[string]verify.Challenge // host -> outstanding challenge
	records    map[string]verify.Record    // host -> verification result
}

// NewVerificationStore initializes an empty, thread-safe verification store.
func NewVerificationStore() *VerificationStore {
	return &VerificationStore{
		challenges: make(map[string]verify.Challenge),
		records:    make(map[string]verify.Record),
	}
}

// TargetStatus is the API-facing view of a target's verification state.
type TargetStatus struct {
	Host       string `json:"host"`
	IsPrivate  bool   `json:"is_private"`
	IsBlocked  bool   `json:"is_blocked"`
	IsVerified bool   `json:"is_verified"`
}

// Status reports the current verification state for a raw target URL/host.
func (v *VerificationStore) Status(rawTarget string) (TargetStatus, error) {
	host, err := verify.ExtractHost(rawTarget)
	if err != nil {
		return TargetStatus{}, err
	}
	v.mu.RLock()
	rec, verified := v.records[host]
	v.mu.RUnlock()

	// If not yet verified in memory, check if a valid DNS TXT record is already published
	if !verified {
		if ch, err := v.Challenge(rawTarget); err == nil {
			if method, err := verify.VerifyChallenge(context.Background(), host, ch.Token); err == nil {
				rec = verify.Record{Host: host, Token: ch.Token, Verified: true, Method: method}
				v.mu.Lock()
				v.records[host] = rec
				v.mu.Unlock()
				verified = true
			}
		}
	}

	return TargetStatus{
		Host:       host,
		IsPrivate:  verify.IsPrivateOrLocalTarget(host),
		IsBlocked:  verify.IsBlockedTarget(host),
		IsVerified: verified && rec.Verified,
	}, nil
}

// Challenge issues (or re-issues) a proof-of-ownership challenge for rawTarget.
func (v *VerificationStore) Challenge(rawTarget string) (verify.Challenge, error) {
	host, err := verify.ExtractHost(rawTarget)
	if err != nil {
		return verify.Challenge{}, err
	}
	if verify.IsBlockedTarget(host) {
		return verify.Challenge{}, fmt.Errorf("%q is a well-known public domain and cannot be verified as a load/chaos test target", host)
	}
	ch, err := verify.GenerateChallenge(host)
	if err != nil {
		return verify.Challenge{}, err
	}
	v.mu.Lock()
	v.challenges[host] = ch
	v.mu.Unlock()
	return ch, nil
}

// Confirm attempts to verify the outstanding challenge for rawTarget via DNS TXT or well-known file.
func (v *VerificationStore) Confirm(ctx context.Context, rawTarget string) (verify.Record, error) {
	host, err := verify.ExtractHost(rawTarget)
	if err != nil {
		return verify.Record{}, err
	}

	v.mu.RLock()
	ch, hasChallenge := v.challenges[host]
	v.mu.RUnlock()
	if !hasChallenge {
		return verify.Record{}, fmt.Errorf("no outstanding challenge for %q — call the challenge endpoint first", host)
	}

	method, err := verify.VerifyChallenge(ctx, host, ch.Token)
	if err != nil {
		return verify.Record{}, err
	}

	rec := verify.Record{Host: host, Token: ch.Token, Verified: true, Method: method}
	v.mu.Lock()
	v.records[host] = rec
	v.mu.Unlock()
	return rec, nil
}

// ConfirmVerified directly marks rawTarget as verified via method, bypassing the DNS/well-known
// challenge flow — used by the cloud-IP-ownership check, whose proof is "the operator's own AWS
// credentials confirm this IP belongs to their account", not a token published on the target.
func (v *VerificationStore) ConfirmVerified(rawTarget string, method verify.Method) (verify.Record, error) {
	host, err := verify.ExtractHost(rawTarget)
	if err != nil {
		return verify.Record{}, err
	}
	rec := verify.Record{Host: host, Verified: true, Method: method}
	v.mu.Lock()
	v.records[host] = rec
	v.mu.Unlock()
	return rec, nil
}

// IsVerified reports whether host currently holds a completed verification.
func (v *VerificationStore) IsVerified(host string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	rec, ok := v.records[host]
	return ok && rec.Verified
}
