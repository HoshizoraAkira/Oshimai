package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSetsEnvVars(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".env")
	os.WriteFile(f, []byte(`
# komentar diabaikan

APP_NAME=oshimai
SECRET_KEY=abc123
QUOTED_DOUBLE="hello world"
QUOTED_SINGLE='foo bar'
`), 0600)

	if err := Load(f); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	cases := map[string]string{
		"APP_NAME":      "oshimai",
		"SECRET_KEY":    "abc123",
		"QUOTED_DOUBLE": "hello world",
		"QUOTED_SINGLE": "foo bar",
	}
	for k, want := range cases {
		if got := os.Getenv(k); got != want {
			t.Errorf("env %q: want %q, got %q", k, want, got)
		}
	}
}

func TestLoadDoesNotOverrideExistingEnv(t *testing.T) {
	os.Setenv("ALREADY_SET", "real-value")
	defer os.Unsetenv("ALREADY_SET")

	dir := t.TempDir()
	f := filepath.Join(dir, ".env")
	os.WriteFile(f, []byte("ALREADY_SET=overridden-value\n"), 0600)

	if err := Load(f); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got := os.Getenv("ALREADY_SET"); got != "real-value" {
		t.Errorf("existing env var was overridden: got %q", got)
	}
}

func TestLoadMissingFileIsNoop(t *testing.T) {
	if err := Load("/tmp/nonexistent-oshimai-dotenv-test-file"); err != nil {
		t.Errorf("expected nil for missing file, got: %v", err)
	}
}

func TestLoadSkipsBlankAndCommentLines(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".env")
	os.WriteFile(f, []byte(`
# ini komentar
   
VALID_KEY=valid_value
# komentar lagi
`), 0600)

	if err := Load(f); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got := os.Getenv("VALID_KEY"); got != "valid_value" {
		t.Errorf("expected 'valid_value', got %q", got)
	}
}
