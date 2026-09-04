package server

import (
	"context"
	"fmt"
	"sync"
)

// StorageRepository defines CRUD persistence contracts for test runs.
type StorageRepository interface {
	Save(ctx context.Context, run *TestRun) error
	Get(ctx context.Context, id string) (*TestRun, error)
	List(ctx context.Context, limit, offset int) ([]*TestRun, error)
	Update(ctx context.Context, run *TestRun) error
	// FindByShareToken looks up a run by its public share token (see the public status page
	// feature). Returns an error if no run currently holds that token.
	FindByShareToken(ctx context.Context, token string) (*TestRun, error)
}

// MemoryRepository provides a high-throughput, thread-safe in-memory repository.
type MemoryRepository struct {
	mu    sync.RWMutex
	runs  map[string]*TestRun
	order []string
}

// NewMemoryRepository initializes the in-memory storage repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		runs:  make(map[string]*TestRun),
		order: make([]string, 0),
	}
}

// Save stores a new test run.
func (r *MemoryRepository) Save(ctx context.Context, run *TestRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("invalid test run or empty ID")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runs[run.ID]; exists {
		return fmt.Errorf("test run %q already exists", run.ID)
	}

	clone := *run
	r.runs[run.ID] = &clone
	r.order = append(r.order, run.ID)
	return nil
}

// Get retrieves a test run by ID.
func (r *MemoryRepository) Get(ctx context.Context, id string) (*TestRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	run, exists := r.runs[id]
	if !exists {
		return nil, fmt.Errorf("test run %q not found", id)
	}

	clone := *run
	return &clone, nil
}

// List returns chronological test runs with pagination support.
func (r *MemoryRepository) List(ctx context.Context, limit, offset int) ([]*TestRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := len(r.order)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []*TestRun{}, nil
	}

	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}

	result := make([]*TestRun, 0, end-offset)
	for i := offset; i < end; i++ {
		id := r.order[i]
		run := r.runs[id]
		clone := *run
		result = append(result, &clone)
	}

	return result, nil
}

// Update updates an existing test run in place.
func (r *MemoryRepository) Update(ctx context.Context, run *TestRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("invalid test run or empty ID")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runs[run.ID]; !exists {
		return fmt.Errorf("test run %q does not exist", run.ID)
	}

	clone := *run
	r.runs[run.ID] = &clone
	return nil
}

// FindByShareToken performs a linear scan for the run holding token. This is intentionally simple
// — share tokens are looked up rarely (once per public-status-page visitor's session) compared to
// the hot Get/List paths, so no secondary index is warranted for an in-memory repository.
func (r *MemoryRepository) FindByShareToken(ctx context.Context, token string) (*TestRun, error) {
	if token == "" {
		return nil, fmt.Errorf("share token cannot be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, id := range r.order {
		run := r.runs[id]
		if run.ShareToken == token {
			clone := *run
			return &clone, nil
		}
	}
	return nil, fmt.Errorf("no run found for share token")
}
