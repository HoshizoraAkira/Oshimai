// Package dotenv provides a minimal .env file loader for development convenience.
// It reads key=value pairs from a file and sets them as environment variables using
// os.Setenv — but only when the variable is NOT already set in the environment.
// This preserves the standard precedence: real env vars always win over .env values,
// which is the correct behaviour for CI/CD and production deployments.
package dotenv

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Load reads the given file path (typically ".env") and populates environment
// variables. Lines that are blank, start with '#', or have no '=' are silently
// skipped. Values may be optionally quoted with single or double quotes.
// Returns nil if the file does not exist — a missing .env is not an error.
func Load(filename string) error {
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		return nil // .env is optional — no error
	}
	if err != nil {
		return fmt.Errorf("dotenv: open %q: %w", filename, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue // no '=' — skip (e.g. export declarations without value)
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Strip optional surrounding quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		// Only set if not already defined — real env always takes priority
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, val); err != nil {
				return fmt.Errorf("dotenv: setenv %q (line %d): %w", key, lineNum, err)
			}
		}
	}
	return scanner.Err()
}
