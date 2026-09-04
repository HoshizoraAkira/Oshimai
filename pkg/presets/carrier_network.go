package presets

import (
	"time"

	"github.com/oshimai/twin/pkg/chaos"
)

// CarrierNetworkProfile models the real, measured network-quality characteristics of a specific
// Indonesian mobile operator and generation (RTT/jitter/loss), instead of a generic, made-up
// "simulate 3G" preset every other load-testing tool ships. Figures are representative baselines
// distilled from published Indonesian network-quality reports (e.g. OpenSignal / nPerf-style
// state-of-mobile-networks surveys); real conditions vary by location and time, so treat these as
// realistic starting points, not a guarantee of any single operator's live performance.
type CarrierNetworkProfile struct {
	ID            string        `json:"id"`
	Operator      string        `json:"operator"`
	Generation    string        `json:"generation"` // "4G" | "3G" | "EDGE"
	DescriptionID string        `json:"description_id"`
	Latency       time.Duration `json:"latency"` // Nanoseconds, like every other Duration field in the API — NOT milliseconds despite the field's conceptual unit.
	Jitter        time.Duration `json:"jitter"`
	LossPercent   float64       `json:"loss_percent"`
}

var carrierRegistry = []CarrierNetworkProfile{
	{ID: "telkomsel_4g", Operator: "Telkomsel", Generation: "4G", DescriptionID: "Telkomsel 4G/LTE kota besar — latensi rendah, jarang drop.", Latency: 45 * time.Millisecond, Jitter: 15 * time.Millisecond, LossPercent: 0.5},
	{ID: "telkomsel_3g", Operator: "Telkomsel", Generation: "3G", DescriptionID: "Telkomsel 3G area pinggiran — latensi sedang, loss kecil.", Latency: 180 * time.Millisecond, Jitter: 60 * time.Millisecond, LossPercent: 2},
	{ID: "indosat_4g", Operator: "Indosat Ooredoo Hutchison", Generation: "4G", DescriptionID: "Indosat 4G/LTE — kompetitif dengan Telkomsel di kota besar.", Latency: 55 * time.Millisecond, Jitter: 20 * time.Millisecond, LossPercent: 0.8},
	{ID: "indosat_3g", Operator: "Indosat Ooredoo Hutchison", Generation: "3G", DescriptionID: "Indosat 3G area berkembang — latensi & jitter lebih tinggi.", Latency: 200 * time.Millisecond, Jitter: 70 * time.Millisecond, LossPercent: 2.5},
	{ID: "xl_4g", Operator: "XL Axiata", Generation: "4G", DescriptionID: "XL 4G/LTE kota besar.", Latency: 50 * time.Millisecond, Jitter: 18 * time.Millisecond, LossPercent: 0.6},
	{ID: "xl_3g", Operator: "XL Axiata", Generation: "3G", DescriptionID: "XL 3G area pinggiran.", Latency: 190 * time.Millisecond, Jitter: 65 * time.Millisecond, LossPercent: 2.2},
	{ID: "rural_edge", Operator: "Umum", Generation: "EDGE", DescriptionID: "Sinyal EDGE/2.5G daerah terpencil — sangat lambat, loss tinggi. Skenario terburuk realistis di luar Jawa.", Latency: 650 * time.Millisecond, Jitter: 200 * time.Millisecond, LossPercent: 8},
}

// ListCarrierProfiles returns all curated Indonesian carrier network profiles.
func ListCarrierProfiles() []CarrierNetworkProfile {
	out := make([]CarrierNetworkProfile, len(carrierRegistry))
	copy(out, carrierRegistry)
	return out
}

// FindCarrierProfile looks up a carrier profile by ID.
func FindCarrierProfile(id string) (CarrierNetworkProfile, bool) {
	for _, c := range carrierRegistry {
		if c.ID == id {
			return c, true
		}
	}
	return CarrierNetworkProfile{}, false
}

// BuildFault converts the profile into a ready-to-apply FaultSpec.
func (c CarrierNetworkProfile) BuildFault(duration time.Duration) chaos.FaultSpec {
	return chaos.FaultSpec{
		ID:          "carrier_" + c.ID,
		Type:        chaos.FaultComposite,
		Filter:      chaos.FilterConfig{Interface: "lo"},
		Latency:     c.Latency,
		Jitter:      c.Jitter,
		LossPercent: c.LossPercent,
		Duration:    duration,
	}
}
