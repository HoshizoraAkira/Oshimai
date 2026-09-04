// Package presets provides load-shape templates calibrated against real Indonesian traffic
// events (flash-sale shopping festivals, payday, national holiday exodus, viral endorsements)
// instead of generic "ramp up / hold / ramp down" defaults. Each preset encodes the *shape* of a
// specific event as a sequence of ramping stages so a non-technical operator can pick "Harbolnas"
// instead of guessing VU counts.
package presets

import (
	"fmt"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
)

// ShapeStage is one ramping stage expressed as percentages, so it scales cleanly across any total
// duration and any baseline VU count — how an operator describes a custom preset's traffic curve
// without writing Go code (see CulturalPreset.Shape).
type ShapeStage struct {
	DurationPercent int     `json:"duration_percent"` // Share of the preset's total duration, 0-100; every stage's share must sum to 100.
	VUMultiplier    float64 `json:"vu_multiplier"`    // Target VUs for this stage = baseline VUs * this multiplier.
}

// CulturalPreset describes a named, pre-shaped load profile tied to a recognizable local event.
// The four built-in presets model Indonesian traffic events; operators elsewhere can add their
// own via Store (Black Friday, Singles' Day 11.11, Boxing Day, a local payday cadence, ...) since
// "what a traffic spike looks like" is not the same shape everywhere.
type CulturalPreset struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	DescriptionID   string       `json:"description_id"` // Bahasa Indonesia explanation shown to the operator.
	DescriptionEN   string       `json:"description_en"`
	PeakMultiplier  int          `json:"peak_multiplier"` // Peak concurrency relative to BaselineVUs.
	SuggestedTotalS int          `json:"suggested_total_duration_sec"`
	Shape           []ShapeStage `json:"shape,omitempty"` // Custom ramp curve; omitted for built-ins (they use a hardcoded shape) and for custom presets happy with the generic ramp/hold/taper default.
	Custom          bool         `json:"is_custom"`       // true for a Store-created preset (editable/deletable); false for the built-ins.
}

var registry = []CulturalPreset{
	{
		ID:              "harbolnas_1212",
		Name:            "Harbolnas 12.12 Flash Sale",
		DescriptionID:   "Lonjakan mendadak di jam 00:00 saat flash sale dibuka, bertahan tinggi ~10 menit, lalu meluruh perlahan. Pola khas trafik Harbolnas/12.12/9.9 e-commerce Indonesia.",
		DescriptionEN:   "A sudden spike at midnight when the flash sale opens, holding near-peak for ~10 minutes before decaying — the classic Indonesian shopping-festival (Harbolnas/12.12/9.9) traffic shape.",
		PeakMultiplier:  12,
		SuggestedTotalS: 300,
	},
	{
		ID:              "gajian_25",
		Name:            "Gajian Tanggal 25 (Payday Rush)",
		DescriptionID:   "Kenaikan bertahap sepanjang jam makan siang saat gajian cair (tanggal 25), dengan dua puncak: siang dan selepas jam kerja.",
		DescriptionEN:   "A gradual climb through lunch hour on payday (the 25th), with two peaks — midday and just after office hours — typical for fintech/e-wallet top-up and e-commerce checkout traffic.",
		PeakMultiplier:  6,
		SuggestedTotalS: 240,
	},
	{
		ID:              "lebaran_mudik",
		Name:            "Lebaran Mudik Exodus",
		DescriptionID:   "Kenaikan trafik yang landai tapi berkepanjangan selama H-7 mudik (tiket, e-wallet, marketplace logistik), tanpa penurunan cepat — beban sustained, bukan spike singkat.",
		DescriptionEN:   "A slow but sustained climb across the week before Lebaran (ticketing, e-wallets, logistics marketplaces) — a long plateau rather than a short spike, testing endurance not just burst capacity.",
		PeakMultiplier:  5,
		SuggestedTotalS: 360,
	},
	{
		ID:              "selebgram_viral",
		Name:            "Endorse Selebgram Viral",
		DescriptionID:   "Lonjakan tajam dan tak terduga begitu story/reels selebgram/TikTok tayang, memuncak dalam hitungan detik lalu turun cepat begitu momen viral lewat.",
		DescriptionEN:   "A sharp, unpredictable spike the moment an influencer's story/reel goes live, peaking within seconds and dropping off quickly once the viral moment passes.",
		PeakMultiplier:  20,
		SuggestedTotalS: 180,
	},
}

// List returns all available cultural load presets.
func List() []CulturalPreset {
	out := make([]CulturalPreset, len(registry))
	copy(out, registry)
	return out
}

// Find looks up a preset by ID.
func Find(id string) (CulturalPreset, bool) {
	for _, p := range registry {
		if p.ID == id {
			return p, true
		}
	}
	return CulturalPreset{}, false
}

// BuildRampingStages expands a CulturalPreset into concrete loadengine.RampingStage steps scaled
// from baselineVUs (the operator's normal/expected concurrent users).
func (p CulturalPreset) BuildRampingStages(baselineVUs int) []loadengine.RampingStage {
	if baselineVUs <= 0 {
		baselineVUs = 10
	}
	peak := baselineVUs * p.PeakMultiplier
	total := time.Duration(p.SuggestedTotalS) * time.Second

	if len(p.Shape) > 0 {
		stages := make([]loadengine.RampingStage, 0, len(p.Shape))
		for _, s := range p.Shape {
			stages = append(stages, loadengine.RampingStage{
				Duration:  total * time.Duration(s.DurationPercent) / 100,
				TargetVUs: int(float64(baselineVUs) * s.VUMultiplier),
			})
		}
		return stages
	}

	switch p.ID {
	case "harbolnas_1212", "selebgram_viral":
		// Sudden spike, short hold, quick decay.
		return []loadengine.RampingStage{
			{Duration: total * 5 / 100, TargetVUs: peak},
			{Duration: total * 55 / 100, TargetVUs: peak},
			{Duration: total * 25 / 100, TargetVUs: baselineVUs * 3},
			{Duration: total * 15 / 100, TargetVUs: baselineVUs},
		}
	case "gajian_25":
		// Two peaks: midday, then after office hours.
		mid := peak
		return []loadengine.RampingStage{
			{Duration: total * 20 / 100, TargetVUs: baselineVUs * 2},
			{Duration: total * 15 / 100, TargetVUs: mid},
			{Duration: total * 15 / 100, TargetVUs: baselineVUs * 2},
			{Duration: total * 20 / 100, TargetVUs: mid},
			{Duration: total * 30 / 100, TargetVUs: baselineVUs},
		}
	case "lebaran_mudik":
		// Slow, sustained climb, long plateau, gentle taper.
		return []loadengine.RampingStage{
			{Duration: total * 20 / 100, TargetVUs: baselineVUs * 2},
			{Duration: total * 20 / 100, TargetVUs: peak * 60 / 100},
			{Duration: total * 40 / 100, TargetVUs: peak},
			{Duration: total * 20 / 100, TargetVUs: baselineVUs},
		}
	default:
		return []loadengine.RampingStage{
			{Duration: total / 4, TargetVUs: peak / 2},
			{Duration: total / 2, TargetVUs: peak},
			{Duration: total / 4, TargetVUs: baselineVUs},
		}
	}
}

// Summary renders a one-line human summary, useful for CLI output and toasts.
func (p CulturalPreset) Summary(baselineVUs int) string {
	return fmt.Sprintf("%s: puncak ~%dx dari beban normal selama %ds", p.Name, p.PeakMultiplier, p.SuggestedTotalS)
}
