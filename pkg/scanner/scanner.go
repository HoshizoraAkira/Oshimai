// Package scanner implements a lightweight, dependency-free static scan for common resiliency
// anti-patterns in a Go codebase — the "pemindai config resiliency di repo" feature: cross-check
// what a load/chaos test found against what the source actually does, instead of only reporting
// symptoms. It is deliberately heuristic (line/window text matching, not a full AST/type-checked
// analysis): it will miss patterns split unusually across lines and can produce the occasional
// false positive, so findings are a starting point for a human to confirm, not a certified audit.
package scanner

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Severity mirrors remediation.IssueSeverity's vocabulary so findings read consistently with the
// rest of Oshimai's reports.
type Severity string

const (
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// Finding is one detected pattern at a specific location.
type Finding struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Snippet  string   `json:"snippet"`
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true, ".next": true,
}

var (
	httpClientLiteralRe    = regexp.MustCompile(`http\.Client\s*\{`)
	timeoutFieldRe         = regexp.MustCompile(`Timeout\s*:`)
	sqlOpenRe              = regexp.MustCompile(`\bsql\.Open\s*\(`)
	setMaxOpenConnsRe      = regexp.MustCompile(`SetMaxOpenConns\s*\(`)
	infiniteForRe          = regexp.MustCompile(`for\s*\{`)
	retryWordRe            = regexp.MustCompile(`(?i)retry|retries|reattempt`)
	retryBoundRe           = regexp.MustCompile(`(?i)maxretries|maxattempts|attempt\s*[<>]|attempts\s*[<>]|i\s*[<>]=?\s*max`)
	backgroundCtxHTTPRe    = regexp.MustCompile(`http\.NewRequestWithContext\(\s*context\.Background\(\)`)
	circuitBreakerImportRe = regexp.MustCompile(`"(github\.com/sony/gobreaker|github\.com/afex/hystrix-go|github\.com/slok/goresilience|github\.com/eapache/go-resiliency)`)
)

// ScanDirectory walks root looking for *.go files and returns every finding, sorted by file then
// line for stable, readable output.
func ScanDirectory(root string) ([]Finding, error) {
	var findings []Finding
	var allSource strings.Builder // Used by repo-wide checks (e.g. "is SetMaxOpenConns used anywhere?").
	type fileLines struct {
		path  string
		lines []string
	}
	var files []fileLines

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Unreadable file: skip rather than fail the whole scan.
		}
		content := string(data)
		allSource.WriteString(content)
		allSource.WriteByte('\n')
		files = append(files, fileLines{path: path, lines: strings.Split(content, "\n")})
		return nil
	})
	if err != nil {
		return nil, err
	}

	repoHasMaxOpenConns := setMaxOpenConnsRe.MatchString(allSource.String())
	repoHasCircuitBreaker := circuitBreakerImportRe.FindStringSubmatch(allSource.String())

	for _, f := range files {
		rel, _ := filepath.Rel(root, f.path)
		if rel == "" {
			rel = f.path
		}

		sawSQLOpen := false
		for i, line := range f.lines {
			lineNo := i + 1

			if httpClientLiteralRe.MatchString(line) {
				window := strings.Join(f.lines[i:min(i+6, len(f.lines))], "\n")
				if !timeoutFieldRe.MatchString(window) {
					findings = append(findings, Finding{
						File: rel, Line: lineNo, Rule: "http-client-no-timeout", Severity: SeverityWarning,
						Message: "http.Client dibuat tanpa Timeout — request bisa menggantung selamanya jika target macet.",
						Snippet: strings.TrimSpace(line),
					})
				}
			}

			if sqlOpenRe.MatchString(line) {
				sawSQLOpen = true
			}

			if backgroundCtxHTTPRe.MatchString(line) {
				findings = append(findings, Finding{
					File: rel, Line: lineNo, Rule: "context-background-in-request", Severity: SeverityWarning,
					Message: "Outbound request pakai context.Background() alih-alih context permintaan asli — cancellation/timeout tidak akan menjalar.",
					Snippet: strings.TrimSpace(line),
				})
			}

			if infiniteForRe.MatchString(line) {
				window := strings.Join(f.lines[i:min(i+15, len(f.lines))], "\n")
				if retryWordRe.MatchString(window) && !retryBoundRe.MatchString(window) {
					findings = append(findings, Finding{
						File: rel, Line: lineNo, Rule: "unbounded-retry-loop", Severity: SeverityWarning,
						Message: "Loop retry tanpa batas maksimum percobaan terdeteksi — berisiko thundering-herd saat dependency down.",
						Snippet: strings.TrimSpace(line),
					})
				}
			}
		}

		if sawSQLOpen && !repoHasMaxOpenConns {
			findings = append(findings, Finding{
				File: rel, Line: 0, Rule: "db-pool-unbounded", Severity: SeverityWarning,
				Message: "sql.Open dipakai tapi SetMaxOpenConns tidak ditemukan di manapun pada repo — pool koneksi DB tidak dibatasi.",
			})
		}
	}

	if len(repoHasCircuitBreaker) > 1 {
		findings = append(findings, Finding{
			File: "", Line: 0, Rule: "circuit-breaker-present", Severity: SeverityInfo,
			Message: "Library circuit breaker terdeteksi (" + repoHasCircuitBreaker[1] + ") — praktik baik sudah diterapkan.",
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})

	return findings, nil
}

// ScanFileContent runs the same line-based rules against a single in-memory Go source file —
// used by the API endpoint, which receives file contents over HTTP rather than a filesystem path.
func ScanFileContent(filename, content string) []Finding {
	var findings []Finding
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNo := i + 1
		if httpClientLiteralRe.MatchString(line) {
			window := strings.Join(lines[i:min(i+6, len(lines))], "\n")
			if !timeoutFieldRe.MatchString(window) {
				findings = append(findings, Finding{File: filename, Line: lineNo, Rule: "http-client-no-timeout", Severity: SeverityWarning,
					Message: "http.Client dibuat tanpa Timeout — request bisa menggantung selamanya jika target macet.", Snippet: strings.TrimSpace(line)})
			}
		}
		if backgroundCtxHTTPRe.MatchString(line) {
			findings = append(findings, Finding{File: filename, Line: lineNo, Rule: "context-background-in-request", Severity: SeverityWarning,
				Message: "Outbound request pakai context.Background() alih-alih context permintaan asli.", Snippet: strings.TrimSpace(line)})
		}
		if infiniteForRe.MatchString(line) {
			window := strings.Join(lines[i:min(i+15, len(lines))], "\n")
			if retryWordRe.MatchString(window) && !retryBoundRe.MatchString(window) {
				findings = append(findings, Finding{File: filename, Line: lineNo, Rule: "unbounded-retry-loop", Severity: SeverityWarning,
					Message: "Loop retry tanpa batas maksimum percobaan terdeteksi.", Snippet: strings.TrimSpace(line)})
			}
		}
	}
	if sqlOpenRe.MatchString(content) && !setMaxOpenConnsRe.MatchString(content) {
		findings = append(findings, Finding{File: filename, Rule: "db-pool-unbounded", Severity: SeverityWarning,
			Message: "sql.Open dipakai tapi SetMaxOpenConns tidak ditemukan di file ini."})
	}
	return findings
}
