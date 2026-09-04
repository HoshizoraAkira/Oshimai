package remediation

import (
	"time"
)

// IssueSeverity indicates the criticality of a detected diagnostic issue.
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "CRITICAL"
	SeverityWarning  IssueSeverity = "WARNING"
	SeverityInfo     IssueSeverity = "INFO"
)

// DetectedIssue encapsulates a specific performance or stability bottleneck found.
type DetectedIssue struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Category    string        `json:"category"`
	Severity    IssueSeverity `json:"severity"`
	Description string        `json:"description"`
}

// BusinessContext supplies commercial parameters so raw error/latency metrics can be translated
// into a business-facing impact estimate (e.g. estimated Rupiah revenue lost during the incident).
type BusinessContext struct {
	AvgTransactionValueIDR         float64 `json:"avg_transaction_value_idr,omitempty"`
	EstimatedTransactionsPerMinute float64 `json:"estimated_transactions_per_minute,omitempty"`
}

// DiagnosticReport is the comprehensive health and remediation assessment produced for a test run.
type DiagnosticReport struct {
	HealthScore          int             `json:"health_score"`           // 1 to 100
	StatusLabel          string          `json:"status_label"`           // e.g. "✅ APLIKASI KUAT & SEHAT"
	SafeCapacityEstimate string          `json:"safe_capacity_estimate"` // e.g. "Aman digunakan hingga 100 pengunjung bersamaan"
	SafeVUCount          int             `json:"safe_vu_count"`
	Summary              string          `json:"summary"`                // Human-friendly summary in casual Indonesian
	RootCause            string          `json:"root_cause"`             // Suspected primary technical root cause
	DetectedIssues       []DetectedIssue `json:"detected_issues"`        // All detected issues
	ActionableFixes      []string        `json:"actionable_fixes"`       // Step-by-step developer fixes
	SuggestedConfigPatch string          `json:"suggested_config_patch"` // Ready-to-use config or code snippet
	GeneratedAt          time.Time       `json:"generated_at"`

	// Business impact translation (populated only when BusinessContext is supplied to Analyze).
	EstimatedRevenueLossIDR float64 `json:"estimated_revenue_loss_idr,omitempty"`
	RevenueLossNote         string  `json:"revenue_loss_note,omitempty"`
}
