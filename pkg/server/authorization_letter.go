package server

import (
	"fmt"
	"html"
)

// GenerateAuthorizationLetter renders a standalone, bilingual "Rules of Engagement" style
// authorization document for a run — scope, time window, target, and approver — analogous to
// what a penetration-testing engagement requires before any test begins, adapted for load/chaos
// testing. It is generated per-run so the paper trail always matches what actually executed.
func GenerateAuthorizationLetter(run *TestRun) string {
	if run == nil {
		return "<html><body>Run not found.</body></html>"
	}

	target := run.Config.TargetBaseURL
	if target == "" {
		target = "(tidak dinyatakan eksplisit — lihat scenario base_url)"
	}
	env := run.Config.Environment
	if env == "" {
		env = "staging/default"
	}
	approver := run.ApprovedBy
	approvedAtStr := "—"
	if approver == "" {
		approver = "(self-service — tidak memerlukan persetujuan kedua)"
	} else {
		approvedAtStr = run.ApprovedAt.Format("2 January 2006, 15:04 WIB")
	}

	chaosLine := "Tidak ada fault injection (chaos) yang dijadwalkan pada run ini."
	if run.Config.ChaosPlan.Enabled {
		chaosLine = fmt.Sprintf(
			"Fault injection dijadwalkan: tipe %s, durasi %s, diterapkan %s setelah mulai.",
			run.Config.ChaosPlan.Fault.Type, run.Config.ChaosPlan.Fault.Duration, run.Config.ChaosPlan.ScheduleDelay,
		)
	}

	windowStart := "belum dimulai"
	if !run.StartTime.IsZero() {
		windowStart = run.StartTime.Format("2 January 2006, 15:04:05 WIB")
	}
	windowEnd := "sedang berjalan / belum selesai"
	if !run.EndTime.IsZero() {
		windowEnd = run.EndTime.Format("2 January 2006, 15:04:05 WIB")
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<title>Surat Otorisasi Uji Beban — %s</title>
<style>
  body { font-family: 'IBM Plex Sans', Arial, sans-serif; color: #161616; max-width: 760px; margin: 40px auto; padding: 0 24px; line-height: 1.6; }
  h1 { font-size: 20px; }
  .eyebrow { font-family: 'IBM Plex Mono', monospace; font-size: 11px; letter-spacing: .08em; color: #0f62fe; text-transform: uppercase; }
  table { width: 100%%; border-collapse: collapse; margin: 20px 0; }
  td, th { border: 1px solid #ccc; padding: 8px 10px; text-align: left; font-size: 13px; vertical-align: top; }
  th { background: #f4f4f4; width: 220px; }
  .sign { display: flex; gap: 40px; margin-top: 48px; }
  .sign div { flex: 1; }
  .sign .line { border-bottom: 1px solid #666; height: 48px; }
  @media print { body { margin: 0; padding: 16px; } }
</style>
</head>
<body>
  <div class="eyebrow">Rules of Engagement // Surat Otorisasi Uji Beban &amp; Chaos</div>
  <h1>Surat Otorisasi Pelaksanaan Uji Beban / Chaos Engineering</h1>
  <p>Dokumen ini mencatat cakupan, jendela waktu, dan pihak yang bertanggung jawab atas eksekusi pengujian beban/chaos berikut, sebagaimana tercatat oleh Oshimai Control Plane.</p>

  <table>
    <tr><th>ID Run</th><td>%s</td></tr>
    <tr><th>Target</th><td>%s</td></tr>
    <tr><th>Environment</th><td>%s</td></tr>
    <tr><th>Jendela Waktu Mulai</th><td>%s</td></tr>
    <tr><th>Jendela Waktu Selesai</th><td>%s</td></tr>
    <tr><th>Profil Beban</th><td>%s — VUs target: %d, Durasi: %s</td></tr>
    <tr><th>Rencana Chaos</th><td>%s</td></tr>
    <tr><th>Disetujui Oleh</th><td>%s</td></tr>
    <tr><th>Waktu Persetujuan</th><td>%s</td></tr>
  </table>

  <p><strong>Pernyataan Cakupan:</strong> Pengujian ini hanya diizinkan mengarah ke target yang telah dinyatakan di atas. Setiap fault injection (chaos) tunduk pada dead-man-switch TTL otomatis dan akan dibatalkan segera jika run diaborsi.</p>

  <div class="sign">
    <div>
      <div class="line"></div>
      <p>Diajukan oleh (Requester)</p>
    </div>
    <div>
      <div class="line"></div>
      <p>Disetujui oleh (Approver / SRE Lead)</p>
    </div>
  </div>
</body>
</html>`,
		html.EscapeString(run.ID), html.EscapeString(run.ID), html.EscapeString(target), html.EscapeString(env),
		windowStart, windowEnd,
		run.Config.LoadConfig.Profile, run.Config.LoadConfig.VUs, run.Config.LoadConfig.Duration,
		html.EscapeString(chaosLine),
		html.EscapeString(approver), approvedAtStr,
	)
}
