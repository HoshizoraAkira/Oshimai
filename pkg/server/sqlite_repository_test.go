package server

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestSQLiteRepo(t *testing.T) (*SQLiteRepository, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	repo, err := NewSQLiteRepository(path)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	return repo, path
}

func TestSQLiteRepository_SaveGetRoundTrip(t *testing.T) {
	repo, _ := newTestSQLiteRepo(t)
	ctx := context.Background()

	run := &TestRun{ID: "run-1", Status: RunStatusQueued}
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "run-1" || got.Status != RunStatusQueued {
		t.Fatalf("Get returned %+v, want ID=run-1 Status=queued", got)
	}
}

func TestSQLiteRepository_SaveDuplicateRejected(t *testing.T) {
	repo, _ := newTestSQLiteRepo(t)
	ctx := context.Background()

	run := &TestRun{ID: "run-1", Status: RunStatusQueued}
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := repo.Save(ctx, run); err == nil {
		t.Fatal("second Save with the same ID should have failed")
	}
}

func TestSQLiteRepository_GetNotFound(t *testing.T) {
	repo, _ := newTestSQLiteRepo(t)
	if _, err := repo.Get(context.Background(), "missing"); err == nil {
		t.Fatal("Get on a missing run should have failed")
	}
}

func TestSQLiteRepository_ListOrderingAndPagination(t *testing.T) {
	repo, _ := newTestSQLiteRepo(t)
	ctx := context.Background()

	for _, id := range []string{"run-1", "run-2", "run-3"} {
		if err := repo.Save(ctx, &TestRun{ID: id, Status: RunStatusQueued}); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}

	all, err := repo.List(ctx, 0, 0)
	if err != nil {
		t.Fatalf("List(0,0): %v", err)
	}
	if len(all) != 3 || all[0].ID != "run-1" || all[2].ID != "run-3" {
		t.Fatalf("List(0,0) = %v, want [run-1 run-2 run-3] in that order", idsOf(all))
	}

	page, err := repo.List(ctx, 2, 1)
	if err != nil {
		t.Fatalf("List(2,1): %v", err)
	}
	if len(page) != 2 || page[0].ID != "run-2" || page[1].ID != "run-3" {
		t.Fatalf("List(2,1) = %v, want [run-2 run-3]", idsOf(page))
	}

	beyond, err := repo.List(ctx, 10, 10)
	if err != nil {
		t.Fatalf("List(10,10): %v", err)
	}
	if len(beyond) != 0 {
		t.Fatalf("List(10,10) = %v, want empty", idsOf(beyond))
	}
}

func idsOf(runs []*TestRun) []string {
	ids := make([]string, len(runs))
	for i, r := range runs {
		ids[i] = r.ID
	}
	return ids
}

func TestSQLiteRepository_Update(t *testing.T) {
	repo, _ := newTestSQLiteRepo(t)
	ctx := context.Background()

	run := &TestRun{ID: "run-1", Status: RunStatusQueued}
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	run.Status = RunStatusCompleted
	if err := repo.Update(ctx, run); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != RunStatusCompleted {
		t.Fatalf("Get after Update returned Status=%s, want completed", got.Status)
	}

	if err := repo.Update(ctx, &TestRun{ID: "missing", Status: RunStatusCompleted}); err == nil {
		t.Fatal("Update on a missing run should have failed")
	}
}

func TestSQLiteRepository_FindByShareToken(t *testing.T) {
	repo, _ := newTestSQLiteRepo(t)
	ctx := context.Background()

	run := &TestRun{ID: "run-1", Status: RunStatusCompleted, ShareToken: "tok-abc"}
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByShareToken(ctx, "tok-abc")
	if err != nil {
		t.Fatalf("FindByShareToken: %v", err)
	}
	if got.ID != "run-1" || got.ShareToken != "tok-abc" {
		t.Fatalf("FindByShareToken returned %+v, want ID=run-1 ShareToken=tok-abc", got)
	}

	if _, err := repo.FindByShareToken(ctx, ""); err == nil {
		t.Fatal("FindByShareToken with an empty token should have failed")
	}
	if _, err := repo.FindByShareToken(ctx, "no-such-token"); err == nil {
		t.Fatal("FindByShareToken with an unknown token should have failed")
	}
}

// TestSQLiteRepository_PersistsAcrossReopen is the test that actually validates the point of this
// repository: data written before the database is closed must still be readable from a fresh
// connection opened against the same file path afterwards, simulating a server restart.
func TestSQLiteRepository_PersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()

	repo1, err := NewSQLiteRepository(path)
	if err != nil {
		t.Fatalf("NewSQLiteRepository (first open): %v", err)
	}
	if err := repo1.Save(ctx, &TestRun{ID: "run-1", Status: RunStatusCompleted, ShareToken: "tok-xyz"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	repo2, err := NewSQLiteRepository(path)
	if err != nil {
		t.Fatalf("NewSQLiteRepository (reopen): %v", err)
	}
	defer repo2.Close()

	got, err := repo2.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.Status != RunStatusCompleted || got.ShareToken != "tok-xyz" {
		t.Fatalf("Get after reopen returned %+v, want Status=completed ShareToken=tok-xyz", got)
	}
}
