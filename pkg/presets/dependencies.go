package presets

import (
	"time"

	"github.com/oshimai/twin/pkg/chaos"
)

// ThirdPartyDependency describes a chaos preset scoped to one well-known Indonesian third-party
// API — payment gateways, maps, and similar — so an operator can answer "what happens to my
// checkout flow if Midtrans goes down for 30 seconds?" without hand-assembling a FaultSpec and a
// domain filter list themselves.
type ThirdPartyDependency struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	DescriptionID string   `json:"description_id"`
	Domains       []string `json:"domains"`
}

var dependencyRegistry = []ThirdPartyDependency{
	{
		ID: "midtrans", Name: "Midtrans (Payment Gateway)",
		DescriptionID: "Simulasikan gateway pembayaran Midtrans lambat/mati — uji apakah checkout kamu gagal dengan anggun atau malah menggantung selamanya.",
		Domains:       []string{"api.midtrans.com", "app.midtrans.com", "api.sandbox.midtrans.com"},
	},
	{
		ID: "xendit", Name: "Xendit (Payment Gateway)",
		DescriptionID: "Simulasikan Xendit lambat/mati untuk menguji fallback pembayaran dan idempotency retry.",
		Domains:       []string{"api.xendit.co"},
	},
	{
		ID: "gopay_ovo", Name: "GoPay / OVO E-Wallet API",
		DescriptionID: "Simulasikan API e-wallet timeout — skenario umum saat trafik nasional e-wallet melonjak (mis. saat promo).",
		Domains:       []string{"api.gopay.co.id", "api.ovo.id"},
	},
	{
		ID: "google_maps", Name: "Google Maps Platform",
		DescriptionID: "Simulasikan Google Maps API (geocoding/rute) lambat — relevan untuk aplikasi logistik & ride-hailing.",
		Domains:       []string{"maps.googleapis.com"},
	},
	{
		ID: "firebase_fcm", Name: "Firebase Cloud Messaging",
		DescriptionID: "Simulasikan FCM lambat/gagal — uji apakah notifikasi push yang gagal terkirim membuat request utama ikut lambat.",
		Domains:       []string{"fcm.googleapis.com"},
	},
}

// ListDependencies returns all curated third-party dependency presets.
func ListDependencies() []ThirdPartyDependency {
	out := make([]ThirdPartyDependency, len(dependencyRegistry))
	copy(out, dependencyRegistry)
	return out
}

// FindDependency looks up a dependency preset by ID.
func FindDependency(id string) (ThirdPartyDependency, bool) {
	for _, d := range dependencyRegistry {
		if d.ID == id {
			return d, true
		}
	}
	return ThirdPartyDependency{}, false
}

// BuildOutageFault builds a domain-scoped FaultSpec simulating this dependency being slow/down:
// high latency plus a meaningful loss percentage, applied only to this dependency's domains so
// the rest of the target application's traffic is untouched.
func (d ThirdPartyDependency) BuildOutageFault(duration time.Duration) chaos.FaultSpec {
	return chaos.FaultSpec{
		ID:          "dependency_outage_" + d.ID,
		Type:        chaos.FaultComposite,
		Filter:      chaos.FilterConfig{Interface: "lo", TargetDomains: d.Domains},
		Latency:     4 * time.Second,
		Jitter:      1 * time.Second,
		LossPercent: 40,
		Duration:    duration,
	}
}
