package server

import "fmt"

// badgeColorForScore mirrors shields.io's traffic-light convention so the badge is legible at a
// glance wherever it's embedded (README, status page, marketing site).
func badgeColorForScore(score int) string {
	switch {
	case score >= 85:
		return "#24a148" // Carbon green-50
	case score >= 60:
		return "#f1c21b" // Carbon yellow-30
	default:
		return "#da1e28" // Carbon red-60
	}
}

// GenerateBadge renders a shields.io-style embeddable SVG badge summarizing a run's health score,
// so a "Battle-Tested by Oshimai" badge can be dropped into a README or landing page the same way
// a CI-status or coverage badge is today.
func GenerateBadge(run *TestRun) string {
	label := "battle-tested by oshimai"
	value := "no data"
	color := "#6f6f6f"

	if run != nil && run.Diagnostics != nil {
		value = fmt.Sprintf("%d/100", run.Diagnostics.HealthScore)
		color = badgeColorForScore(run.Diagnostics.HealthScore)
	}

	const labelWidth = 148 // Fixed width tuned for the "battle-tested by oshimai" phrase above.
	valueWidth := 54 + len(value)*6
	totalWidth := labelWidth + valueWidth

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">
  <linearGradient id="s" x2="0" y2="100%%">
    <stop offset="0" stop-color="#fff" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r"><rect width="%d" height="20" rx="3" fill="#fff"/></clipPath>
  <g clip-path="url(#r)">
    <rect width="%d" height="20" fill="#161616"/>
    <rect x="%d" width="%d" height="20" fill="%s"/>
    <rect width="%d" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="IBM Plex Mono,monospace" font-size="11">
    <text x="%d" y="14">%s</text>
    <text x="%d" y="14">%s</text>
  </g>
</svg>`,
		totalWidth, label, value,
		totalWidth,
		totalWidth,
		labelWidth, valueWidth, color,
		totalWidth,
		labelWidth/2, label,
		labelWidth+valueWidth/2, value,
	)
}
