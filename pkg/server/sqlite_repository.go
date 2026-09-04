package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

// sqliteSchema stores every run as an ID-addressable row with the run itself as a JSON blob
// (TestRun's shape — nested Summary/Diagnostics/Scenario structs) rather than a normalized
// schema: the API only ever needs to fetch a run by ID/share-token or page through them in
// insertion order, so a blob plus two indexed lookup columns covers every access pattern
// StorageRepository needs without a migration for every field TestRun gains over time.
const sqliteSchema = `
CREATE TABLE IF NOT EXISTS runs (
	seq         INTEGER PRIMARY KEY AUTOINCREMENT,
	id          TEXT NOT NULL UNIQUE,
	status      TEXT NOT NULL,
	share_token TEXT NOT NULL DEFAULT '',
	data        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_runs_share_token ON runs(share_token) WHERE share_token != '';
`

// SQLiteRepository persists test runs to a SQLite database file so run history, diagnostics, and
// share links survive a server restart — unlike MemoryRepository, whose data lives only as long
// as the process does.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository opens (creating if necessary) a SQLite database at path and prepares its
// schema. SQLite allows only one writer at a time; a single connection avoids "database is
// locked" errors under concurrent access instead of masking them with busy-retry logic.
func NewSQLiteRepository(path string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize sqlite schema at %q: %w", path, err)
	}
	return &SQLiteRepository{db: db}, nil
}

// Close releases the underlying database connection.
func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

// Save stores a new test run.
func (r *SQLiteRepository) Save(ctx context.Context, run *TestRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("invalid test run or empty ID")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to save test run %q: %w", run.ID, err)
	}
	defer tx.Rollback()

	var exists int
	switch err := tx.QueryRowContext(ctx, `SELECT 1 FROM runs WHERE id = ?`, run.ID).Scan(&exists); err {
	case nil:
		return fmt.Errorf("test run %q already exists", run.ID)
	case sql.ErrNoRows:
		// Expected — no existing row, safe to insert.
	default:
		return fmt.Errorf("failed to save test run %q: %w", run.ID, err)
	}

	data, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("failed to encode test run %q: %w", run.ID, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO runs (id, status, share_token, data) VALUES (?, ?, ?, ?)`,
		run.ID, string(run.Status), run.ShareToken, data,
	); err != nil {
		return fmt.Errorf("failed to save test run %q: %w", run.ID, err)
	}
	return tx.Commit()
}

// Get retrieves a test run by ID.
func (r *SQLiteRepository) Get(ctx context.Context, id string) (*TestRun, error) {
	var data []byte
	var shareToken string
	err := r.db.QueryRowContext(ctx, `SELECT data, share_token FROM runs WHERE id = ?`, id).Scan(&data, &shareToken)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("test run %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load test run %q: %w", id, err)
	}
	return decodeTestRun(data, shareToken)
}

// List returns chronological test runs with pagination support. limit<=0 means "no limit",
// matching MemoryRepository's semantics.
func (r *SQLiteRepository) List(ctx context.Context, limit, offset int) ([]*TestRun, error) {
	if offset < 0 {
		offset = 0
	}
	sqlLimit := limit
	if sqlLimit <= 0 {
		sqlLimit = -1 // SQLite's idiom for "unlimited" with LIMIT/OFFSET.
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT data, share_token FROM runs ORDER BY seq ASC LIMIT ? OFFSET ?`, sqlLimit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list test runs: %w", err)
	}
	defer rows.Close()

	result := []*TestRun{}
	for rows.Next() {
		var data []byte
		var shareToken string
		if err := rows.Scan(&data, &shareToken); err != nil {
			return nil, fmt.Errorf("failed to list test runs: %w", err)
		}
		run, err := decodeTestRun(data, shareToken)
		if err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list test runs: %w", err)
	}
	return result, nil
}

// Update updates an existing test run in place.
func (r *SQLiteRepository) Update(ctx context.Context, run *TestRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("invalid test run or empty ID")
	}

	data, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("failed to encode test run %q: %w", run.ID, err)
	}

	result, err := r.db.ExecContext(ctx,
		`UPDATE runs SET status = ?, share_token = ?, data = ? WHERE id = ?`,
		string(run.Status), run.ShareToken, data, run.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update test run %q: %w", run.ID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to update test run %q: %w", run.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("test run %q does not exist", run.ID)
	}
	return nil
}

// FindByShareToken looks up a run by its public share token.
func (r *SQLiteRepository) FindByShareToken(ctx context.Context, token string) (*TestRun, error) {
	if token == "" {
		return nil, fmt.Errorf("share token cannot be empty")
	}

	var data []byte
	err := r.db.QueryRowContext(ctx, `SELECT data FROM runs WHERE share_token = ? LIMIT 1`, token).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no run found for share token")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to look up share token: %w", err)
	}
	return decodeTestRun(data, token)
}

// decodeTestRun unmarshals a stored TestRun's JSON blob and restores ShareToken, which — per
// TestRun.ShareToken's `json:"-"` tag — is deliberately excluded from the JSON encoding (so it
// never leaks through the regular run API responses) and therefore must be threaded back in from
// the dedicated share_token column instead of round-tripping through the blob like every other field.
func decodeTestRun(data []byte, shareToken string) (*TestRun, error) {
	var run TestRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("failed to decode stored test run: %w", err)
	}
	run.ShareToken = shareToken
	return &run, nil
}
