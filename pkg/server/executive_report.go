package server

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// GenerateExecutiveReport renders a standalone, print-ready, bilingual (ID/EN) one-page HTML
// report summarizing a run for a non-technical stakeholder. It is deliberately a plain HTML
// document with print CSS rather than a PDF library dependency: opening it and choosing
// "Print > Save as PDF" produces an equivalent artifact without adding a rendering dependency.
func GenerateExecutiveReport(run *TestRun) string {
	if run == nil {
		return "<html><body>Run not found.</body></html>"
	}

	diag := run.Diagnostics
	summary := run.Summary

	score := 0
	statusLabel := "N/A"
	safeCapacity := "N/A"
	businessSummary := "Belum ada diagnosis tersedia untuk run ini."
	revenueLine := ""
	fixesHTML := "<li>Tidak ada temuan.</li>"

	if diag != nil {
		score = diag.HealthScore
		statusLabel = diag.StatusLabel
		safeCapacity = diag.SafeCapacityEstimate
		businessSummary = diag.Summary
		if diag.EstimatedRevenueLossIDR > 0 {
			revenueLine = fmt.Sprintf(`<div class="callout callout-danger">
				<strong>Estimasi Dampak Bisnis:</strong> ~Rp%s per insiden serupa.<br/>
				<span class="muted">%s</span>
			</div>`, formatRupiah(diag.EstimatedRevenueLossIDR), html.EscapeString(diag.RevenueLossNote))
		}
		if len(diag.ActionableFixes) > 0 {
			var sb strings.Builder
			for _, f := range diag.ActionableFixes {
				sb.WriteString("<li>" + html.EscapeString(f) + "</li>")
			}
			fixesHTML = sb.String()
		}
	}

	totalReqs, actualRPS, p99Ms := int64(0), 0.0, 0.0
	if summary != nil {
		totalReqs = summary.TotalRequests
		actualRPS = summary.ActualRPS
		p99Ms = float64(summary.Latency.P99.Microseconds()) / 1000.0
	}

	generatedAt := time.Now().Format("2 January 2006, 15:04 WIB")

	return fmt.Sprintf(`<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<title>Laporan Eksekutif — %s</title>
<style>
  body { font-family: 'IBM Plex Sans', Arial, sans-serif; color: #161616; max-width: 780px; margin: 40px auto; padding: 0 24px; }
  h1 { font-size: 22px; border-bottom: 3px solid #0f62fe; padding-bottom: 12px; }
  .eyebrow { font-family: 'IBM Plex Mono', monospace; font-size: 11px; letter-spacing: .08em; color: #0f62fe; text-transform: uppercase; }
  .grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin: 20px 0; }
  .tile { border: 1px solid #ddd; padding: 12px; }
  .tile .label { font-size: 10px; text-transform: uppercase; color: #666; }
  .tile .value { font-size: 20px; font-weight: 600; }
  .callout { border-left: 4px solid #0f62fe; background: #f4f4f4; padding: 12px 16px; margin: 16px 0; }
  .callout-danger { border-left-color: #da1e28; background: #fff1f1; }
  .muted { color: #666; font-size: 12px; }
  ul { padding-left: 18px; }
  footer { margin-top: 32px; font-size: 11px; color: #888; border-top: 1px solid #ddd; padding-top: 12px; }
  @media print { body { margin: 0; padding: 16px; } }
</style>
</head>
<body>
  <div class="eyebrow">Oshimai Executive Summary // Ringkasan Eksekutif</div>
  <h1>Hasil Uji Beban &amp; Ketahanan — %s</h1>
  <p class="muted">Run ID: %s &middot; Dibuat %s</p>

  <div class="grid">
    <div class="tile"><div class="label">Skor Kesehatan</div><div class="value">%d/100</div></div>
    <div class="tile"><div class="label">Total Request</div><div class="value">%d</div></div>
    <div class="tile"><div class="label">Throughput</div><div class="value">%.1f rps</div></div>
  </div>

  <p><strong>%s</strong></p>
  <p>%s</p>
  <p><strong>Kapasitas Aman:</strong> %s</p>
  <p><strong>Latensi P99:</strong> %.1f ms</p>

  %s

  <h3>Rekomendasi Tindak Lanjut</h3>
  <ul>%s</ul>

  <footer>Dihasilkan otomatis oleh Oshimai Autonomous Chaos &amp; Load-Testing Twin. Dokumen ini dapat dicetak/disimpan sebagai PDF (Ctrl/Cmd+P).</footer>
</body>
</html>`,
		html.EscapeString(run.ID), html.EscapeString(run.ID), html.EscapeString(run.ID), generatedAt,
		score, totalReqs, actualRPS,
		html.EscapeString(statusLabel), html.EscapeString(businessSummary), html.EscapeString(safeCapacity), p99Ms,
		revenueLine, fixesHTML,
	)
}

func formatRupiah(amount float64) string {
	s := fmt.Sprintf("%.0f", amount)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, d := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, d)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
