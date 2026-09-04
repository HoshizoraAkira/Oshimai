package remediation

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
)

// Analyze evaluates execution results, pinpoints bottlenecks, and generates a human-friendly diagnostic report.
// biz is optional (nil-able); when supplied, the report is extended with an estimated Rupiah revenue-loss
// translation so non-technical stakeholders can read failures in business terms instead of error rates.
func Analyze(summary *loadengine.ExecutionSummary, targetVUs int, isChaos bool, biz *BusinessContext) *DiagnosticReport {
	if targetVUs <= 0 {
		targetVUs = 10
	}

	report := &DiagnosticReport{
		GeneratedAt:     time.Now(),
		DetectedIssues:  make([]DetectedIssue, 0),
		ActionableFixes: make([]string, 0),
	}

	if summary == nil || summary.TotalRequests == 0 {
		report.HealthScore = 50
		report.StatusLabel = "⚠️ TIDAK ADA DATA PENGUJIAN"
		report.SafeCapacityEstimate = "Belum dapat ditentukan (tidak ada request yang tercatat)"
		report.SafeVUCount = 0
		report.Summary = "Pengujian tidak menghasilkan request yang berhasil dikirim ke server. Pastikan Base URL target dapat diakses dari lingkungan ini."
		report.RootCause = "Koneksi ke target URL gagal sebelum pengujian beban berlangsung (misal salah alamat URL, port tertutup, atau DNS tidak ditemukan)."
		report.ActionableFixes = []string{
			"Periksa kembali Base URL target apakah sudah benar dan dapat dijangkau.",
			"Pastikan server target aktif dan port yang dituju terbuka.",
			"Coba lakukan ping / curl manual ke endpoint target untuk memastikan konektivitas.",
		}
		report.SuggestedConfigPatch = "curl -v <TARGET_BASE_URL>/health"
		return report
	}

	// 1. Compute Key Metrics
	totalReqs := summary.TotalRequests
	totalErrors := summary.TotalErrors
	errorRate := float64(totalErrors) / float64(totalReqs)
	p99Ms := float64(summary.Latency.P99.Microseconds()) / 1000.0
	meanMs := float64(summary.Latency.Mean.Microseconds()) / 1000.0

	statusCodes := summary.StatusCodes
	if statusCodes == nil {
		statusCodes = make(map[int]int64)
	}

	count504 := statusCodes[504] + statusCodes[408]
	count502 := statusCodes[502] + statusCodes[500] + statusCodes[503]
	count429 := statusCodes[429]

	isAbortedByCB := summary.TerminationStatus == loadengine.StatusAbortedByCircuitBreaker

	// 2. Health Score Calculation (1 to 100)
	score := 100.0

	// Error penalty (up to 55 points)
	if errorRate > 0 {
		errPenalty := errorRate * 100.0 * 1.5
		if errPenalty > 55.0 {
			errPenalty = 55.0
		}
		score -= errPenalty
	}

	// Latency P99 penalty (up to 30 points)
	if p99Ms > 3000.0 {
		score -= 30.0
	} else if p99Ms > 1500.0 {
		score -= 20.0
	} else if p99Ms > 800.0 {
		score -= 10.0
	} else if p99Ms > 400.0 {
		score -= 5.0
	}

	// Circuit Breaker abort penalty
	if isAbortedByCB {
		score -= 20.0
	}

	// Severe crash penalty (502/500)
	if count502 > 0 {
		ratio502 := float64(count502) / float64(totalReqs)
		if ratio502 > 0.10 {
			score -= 15.0
		} else {
			score -= 8.0
		}
	}

	finalScore := int(math.Round(score))
	if finalScore < 5 {
		finalScore = 5
	}
	if finalScore > 100 {
		finalScore = 100
	}
	report.HealthScore = finalScore

	// 3. Status Label & Safe Concurrency Estimation
	if finalScore >= 85 {
		report.StatusLabel = "✅ APLIKASI KUAT & SEHAT"
		report.SafeVUCount = targetVUs
		report.SafeCapacityEstimate = fmt.Sprintf("Aman digunakan hingga %d pengunjung bersamaan tanpa kendala.", targetVUs)
	} else if finalScore >= 60 {
		safeCount := int(float64(targetVUs) * 0.70)
		if safeCount < 1 {
			safeCount = 1
		}
		report.SafeVUCount = safeCount
		report.StatusLabel = fmt.Sprintf("⚠️ APLIKASI MULAI MACET DI %d USER", targetVUs)
		report.SafeCapacityEstimate = fmt.Sprintf("Aman digunakan hingga ~%d pengunjung bersamaan. Di atas ini respons mulai melambat.", safeCount)
	} else {
		safeCount := int(float64(targetVUs) * (float64(finalScore) / 120.0))
		if safeCount < 1 {
			safeCount = 1
		}
		report.SafeVUCount = safeCount
		if isAbortedByCB {
			report.StatusLabel = "🚨 APLIKASI TUMBANG (CIRCUIT BREAKER AKTIF)"
		} else {
			report.StatusLabel = "🚨 APLIKASI RENTAN CRASH"
		}
		report.SafeCapacityEstimate = fmt.Sprintf("Kapasitas aman kritis (~%d pengunjung bersamaan). Butuh perbaikan segera sebelum rilis publik.", safeCount)
	}

	// 4. Pattern Detection
	var primaryRootCause string
	var summaryText string
	var configPatches []string

	// Pattern 1: HTTP 504 / 408 Gateway Timeout
	hasTimeout := count504 > 0 || strings.Contains(strings.ToLower(summary.AbortReason), "timeout")
	if hasTimeout {
		report.DetectedIssues = append(report.DetectedIssues, DetectedIssue{
			ID:          "ISSUE-504-TIMEOUT",
			Title:       "Gateway Timeout (HTTP 504 / 408)",
			Category:    "Database & Upstream",
			Severity:    SeverityCritical,
			Description: fmt.Sprintf("Ditemukan %d request timeout. Server atau reverse proxy kehabisan waktu menunggu response dari database atau microservice internal.", count504),
		})
		report.ActionableFixes = append(report.ActionableFixes,
			"Analisis query database lambat menggunakan `EXPLAIN ANALYZE` dan tambahkan indeks yang relevan.",
			"Gunakan Redis caching untuk endpoint read-heavy yang sering diakses berulang.",
			"Atur downstream timeout yang realistis dengan pola circuit breaker di level service mesh / reverse proxy.",
		)
		configPatches = append(configPatches, `-- Tambahkan Index pada query database yang sering dicari/difilter
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_records_lookup ON table_name (status, created_at DESC);

-- Contoh optimasi timeout pada Reverse Proxy (Nginx)
proxy_connect_timeout 5s;
proxy_read_timeout 10s;
proxy_send_timeout 10s;`)
		if primaryRootCause == "" {
			primaryRootCause = "Query database lambat atau downstream service timeout saat melayani lonjakan request."
			summaryText = fmt.Sprintf("Server kamu kewalahan saat melayani %d user bersamaan karena transaksi memakan waktu terlalu lama hingga terjadi timeout.", targetVUs)
		}
	}

	// Pattern 2: HTTP 502 / 500 / Connection Refused (Server Crash / OOM)
	hasCrash := count502 > 0 || strings.Contains(strings.ToLower(summary.AbortReason), "connection refused")
	if hasCrash {
		report.DetectedIssues = append(report.DetectedIssues, DetectedIssue{
			ID:          "ISSUE-502-CRASH",
			Title:       "Server Crash / Bad Gateway (HTTP 502 / 500)",
			Category:    "Runtime Stability",
			Severity:    SeverityCritical,
			Description: fmt.Sprintf("Ditemukan %d kegagalan fatal server. Kemungkinan proses aplikasi backend mati mendadak, mengalami OOM (Out Of Memory), atau panic.", count502),
		})
		report.ActionableFixes = append(report.ActionableFixes,
			"Periksa log container atau kernel via `kubectl describe pod` atau `dmesg -T | grep -i oom` untuk mencari indikasi OOMKilled.",
			"Naikkan resource memory limits pada konfigurasi Kubernetes deployment atau Docker Compose.",
			"Lakukan profiling memory leak menggunakan Go pprof (`go tool pprof http://localhost:6060/debug/pprof/heap`).",
		)
		configPatches = append(configPatches, `# Konfigurasi Resource Limits Kubernetes (Cegah OOMKilled)
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "2000m"`)
		if primaryRootCause == "" {
			primaryRootCause = "Proses web server crash, kehabisan memori (OOM), atau container restart di bawah beban tinggi."
			summaryText = fmt.Sprintf("Aplikasi backend sempat tumbang atau me-restart saat diuji dengan %d user. Kemungkinan server kehabisan memori (OOM) atau kehabisan thread worker.", targetVUs)
		}
	}

	// Pattern 3: HTTP 429 Too Many Requests (Rate Limiter)
	if count429 > 0 {
		report.DetectedIssues = append(report.DetectedIssues, DetectedIssue{
			ID:          "ISSUE-429-RATELIMIT",
			Title:       "Rate Limiter Terpicu (HTTP 429)",
			Category:    "Traffic Protection",
			Severity:    SeverityWarning,
			Description: fmt.Sprintf("Tercatat %d request dibatasi oleh sistem proteksi rate limiter (HTTP 429 Too Many Requests).", count429),
		})
		report.ActionableFixes = append(report.ActionableFixes,
			"Jika ini adalah proteksi keamanan yang memang sengaja dipasang: Selamat, rate-limiter kamu berhasil melindungi server dari overload!",
			"Jika ini false-positive untuk traffic pelanggan sah: Naikkan kuota rate limit dan burst rate pada reverse proxy / API gateway.",
			"Kirimkan header `Retry-After` pada respon 429 agar aplikasi mobile/frontend dapat menunggu sebelum mencoba kembali.",
		)
		configPatches = append(configPatches, `# Konfigurasi Rate Limit Nginx (Tingkatkan batas burst jika perlu)
limit_req_zone $binary_remote_addr zone=api_zone:10m rate=100r/s;
location /api/ {
    limit_req zone=api_zone burst=200 nodelay;
}`)
		if primaryRootCause == "" {
			primaryRootCause = "Rate-limiter target aktif membatasi lonjakan request atau kuota API terlampaui."
			summaryText = fmt.Sprintf("Aplikasi membatasi traffic masuk dengan HTTP 429. Mekanisme perlindungan rate-limiter kamu aktif dan berfungsi menahan beban %d user.", targetVUs)
		}
	}

	// Pattern 4: Latency Surge under load (Thread / Connection Pool Exhaustion)
	hasLatencySpike := (p99Ms > 1200.0) || (targetVUs >= 50 && p99Ms > 800.0)
	if hasLatencySpike && !hasTimeout {
		report.DetectedIssues = append(report.DetectedIssues, DetectedIssue{
			ID:          "ISSUE-LATENCY-SPIKE",
			Title:       "Latensi Melonjak saat Beban Naik",
			Category:    "Connection & Thread Pool",
			Severity:    SeverityWarning,
			Description: fmt.Sprintf("Latensi P99 menyentuh %.1f ms (Rata-rata: %.1f ms). Waktu respon melambat signifikan saat jumlah user bertambah.", p99Ms, meanMs),
		})
		report.ActionableFixes = append(report.ActionableFixes,
			"Perbesar ukuran connection pool database (`SetMaxOpenConns` & `SetMaxIdleConns`) di aplikasi.",
			"Gunakan connection pooler seperti PgBouncer untuk PostgreSQL jika database mendekati max_connections.",
			"Pisahkan beban transaksi berat ke antrean background worker (asynchronous queue via Redis/RabbitMQ).",
		)
		configPatches = append(configPatches, `// Contoh Penyetelan Connection Pool Database di Go:
db.SetMaxOpenConns(100) // Sesuaikan dengan batas server database
db.SetMaxIdleConns(25)
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(1 * time.Minute)`)
		if primaryRootCause == "" {
			primaryRootCause = "Database connection pool kekecilan atau CPU worker pool mengalami bottleneck saat konkurensi naik."
			summaryText = fmt.Sprintf("Aplikasi masih melayani seluruh request, tapi mulai melambat ketika diuji dengan %d user karena antrean proses menumpuk.", targetVUs)
		}
	}

	// Pattern 5: Chaos / Simulated Poor Connection
	if isChaos {
		report.DetectedIssues = append(report.DetectedIssues, DetectedIssue{
			ID:          "ISSUE-CHAOS-3G",
			Title:       "Simulasi Jaringan Lemot (3G/Loss)",
			Category:    "Network Resilience",
			Severity:    SeverityInfo,
			Description: "Pengujian dijalankan dengan simulasi sinyal buruk/paket loss. Menunjukkan ketahanan aplikasi terhadap pelanggan dengan koneksi seluler lambat.",
		})
		report.ActionableFixes = append(report.ActionableFixes,
			"Aktifkan kompresi Gzip/Brotli pada web server untuk memangkas ukuran byte respons.",
			"Gunakan HTTP/2 atau HTTP/3 (QUIC) untuk mengurangi latensi handshake dan packet loss TCP.",
		)
		configPatches = append(configPatches, `# Konfigurasi Gzip Compression di Web Server:
gzip on;
gzip_types application/json text/plain text/css application/javascript;
gzip_min_length 1000;`)
	}

	// Healthy Baseline
	if len(report.DetectedIssues) == 0 && finalScore >= 85 {
		report.Summary = fmt.Sprintf("Aplikasi kamu sangat kuat dan sehat! Sukses melayani %d pengunjung bersamaan dengan kecepatan rata-rata %.1f ms (P99: %.1f ms) tanpa hambatan.", targetVUs, meanMs, p99Ms)
		report.RootCause = "Tidak ditemukan bottleneck. Kapasitas server, koneksi database, dan memori saat ini sangat memadai untuk beban pengujian ini."
		report.ActionableFixes = []string{
			"Pertahankan performa ini dan integrasikan Oshimai ke pipeline CI/CD untuk otomatisasi uji regresi performa.",
			"Pasang monitoring metrik (Prometheus/Grafana) untuk memantau performa real-time di lingkungan produksi.",
			"Coba uji dengan preset 'Flash Sale (500 User)' untuk mengetahui batas absolut daya tahan server kamu.",
		}
		report.SuggestedConfigPatch = `# Konfigurasi Health Check Liveness Probe Kubernetes:
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10`
	} else {
		report.Summary = summaryText
		report.RootCause = primaryRootCause
		if len(configPatches) > 0 {
			report.SuggestedConfigPatch = strings.Join(configPatches, "\n\n")
		}
	}

	// 5. Business impact translation (only when the caller supplies transaction economics).
	if biz != nil && biz.AvgTransactionValueIDR > 0 {
		failedTransactions := float64(totalErrors)
		if biz.EstimatedTransactionsPerMinute > 0 {
			minutesElapsed := summary.TotalDuration.Minutes()
			if minutesElapsed <= 0 {
				minutesElapsed = 1
			}
			projectedTransactions := biz.EstimatedTransactionsPerMinute * minutesElapsed
			failedTransactions = projectedTransactions * errorRate
		}
		report.EstimatedRevenueLossIDR = failedTransactions * biz.AvgTransactionValueIDR
		report.RevenueLossNote = fmt.Sprintf(
			"Berdasarkan %.0f%% error rate dan rata-rata nilai transaksi Rp%.0f, insiden seperti ini berpotensi menghilangkan ~Rp%.0f pendapatan setiap kali terjadi di produksi.",
			errorRate*100, biz.AvgTransactionValueIDR, report.EstimatedRevenueLossIDR,
		)
	}

	return report
}
