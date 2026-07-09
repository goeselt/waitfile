package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_noArgs(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr missing Usage, got: %s", stderr.String())
	}
}

func TestRun_help(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr missing Usage, got: %s", stderr.String())
	}
}

func TestRun_unknownFlag(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"--verbose"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRun_fileAlreadyExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "exists.txt") {
		t.Errorf("stdout missing filename, got: %s", stdout.String())
	}
}

func TestRun_timeout(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-t", "1", "/nonexistent/path/xyz"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestRun_negativeTimeout(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-t", "-5", "/nonexistent/path/xyz"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "invalid timeout") {
		t.Errorf("stderr missing 'invalid timeout', got: %s", stderr.String())
	}
}

func TestExitCode_mapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		index int
		want  int
	}{
		{0, 0},
		{1, 10},
		{2, 11},
		{3, 12},
		{4, 13},
	}

	for _, tt := range tests {
		got := exitCode(tt.index)
		if got != tt.want {
			t.Errorf("exitCode(%d) = %d, want %d", tt.index, got, tt.want)
		}
	}
}
