package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExitCode(t *testing.T) {
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
		if got := exitCode(tt.index); got != tt.want {
			t.Errorf("exitCode(%d) = %d, want %d", tt.index, got, tt.want)
		}
	}
}

func TestWaitForFileAlreadyExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "ready.marker")

	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatalf("create marker: %v", err)
	}

	idx, err := waitForFile([]string{target}, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 {
		t.Errorf("got index %d, want 0", idx)
	}
}

func TestWaitForFileCreatedAfterWatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "created.marker")

	done := make(chan struct{})
	var resultIdx int
	var resultErr error

	go func() {
		resultIdx, resultErr = waitForFile([]string{target}, time.Now().Add(5*time.Second), nil)
		close(done)
	}()

	// Give the watcher time to set up inotify.
	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatalf("create marker: %v", err)
	}

	<-done

	if resultErr != nil {
		t.Fatalf("unexpected error: %v", resultErr)
	}
	if resultIdx != 0 {
		t.Errorf("got index %d, want 0", resultIdx)
	}
}

func TestWaitForFileTimeout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "never.marker")

	_, err := waitForFile([]string{target}, time.Now().Add(100*time.Millisecond), nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if err.Error() != "timeout" {
		t.Errorf("got error %q, want %q", err.Error(), "timeout")
	}
}

func TestWaitForFileMultiplePaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first := filepath.Join(dir, "first.marker")
	second := filepath.Join(dir, "second.marker")

	done := make(chan struct{})
	var resultIdx int
	var resultErr error

	go func() {
		resultIdx, resultErr = waitForFile([]string{first, second}, time.Now().Add(5*time.Second), nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Create the second path first - should return index 1.
	if err := os.WriteFile(second, []byte("ok"), 0o644); err != nil {
		t.Fatalf("create marker: %v", err)
	}

	<-done

	if resultErr != nil {
		t.Fatalf("unexpected error: %v", resultErr)
	}
	if resultIdx != 1 {
		t.Errorf("got index %d, want 1", resultIdx)
	}
}

func TestWaitForFileAlreadyExistsSecondPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first := filepath.Join(dir, "first.marker")
	second := filepath.Join(dir, "second.marker")

	// Only second path exists.
	if err := os.WriteFile(second, []byte("ok"), 0o644); err != nil {
		t.Fatalf("create marker: %v", err)
	}

	idx, err := waitForFile([]string{first, second}, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 {
		t.Errorf("got index %d, want 1", idx)
	}
}

func TestWaitForFileParentDirCreatedLater(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	// Target lives in a subdirectory that does not exist yet.
	subdir := filepath.Join(base, "subdir")
	target := filepath.Join(subdir, "ready.marker")

	done := make(chan struct{})
	var resultIdx int
	var resultErr error

	go func() {
		resultIdx, resultErr = waitForFile([]string{target}, time.Now().Add(5*time.Second), nil)
		close(done)
	}()

	// Give the watcher time to start polling for the parent directory.
	time.Sleep(500 * time.Millisecond)

	// Create the parent directory, then the target file.
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatalf("create marker: %v", err)
	}

	<-done

	if resultErr != nil {
		t.Fatalf("unexpected error: %v", resultErr)
	}
	if resultIdx != 0 {
		t.Errorf("got index %d, want 0", resultIdx)
	}
}

func TestWaitForFileParentDirMissingTimeout(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	// Parent directory never gets created -> must time out.
	target := filepath.Join(base, "nonexistent", "never.marker")

	_, err := waitForFile([]string{target}, time.Now().Add(500*time.Millisecond), nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if err.Error() != "timeout" {
		t.Errorf("got error %q, want %q", err.Error(), "timeout")
	}
}

// --- Tests for -nonempty (contentOK) ---

func TestNonemptyAlreadyExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "ready.marker")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("create marker: %v", err)
	}

	idx, err := waitForFile([]string{target}, time.Time{}, isNonEmpty)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 {
		t.Errorf("got index %d, want 0", idx)
	}
}

func TestNonemptyExistsButEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "empty.marker")

	// Create an empty file -- should not match yet.
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatalf("create empty marker: %v", err)
	}

	done := make(chan struct{})
	var resultIdx int
	var resultErr error

	go func() {
		resultIdx, resultErr = waitForFile([]string{target}, time.Now().Add(5*time.Second), isNonEmpty)
		close(done)
	}()

	// Give the watcher time to set up inotify.
	time.Sleep(50 * time.Millisecond)

	// Write content -- should now match.
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("write content: %v", err)
	}

	<-done

	if resultErr != nil {
		t.Fatalf("unexpected error: %v", resultErr)
	}
	if resultIdx != 0 {
		t.Errorf("got index %d, want 0", resultIdx)
	}
}

func TestNonemptyCreatedEmptyThenWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "delayed.marker")

	done := make(chan struct{})
	var resultIdx int
	var resultErr error

	go func() {
		resultIdx, resultErr = waitForFile([]string{target}, time.Now().Add(5*time.Second), isNonEmpty)
		close(done)
	}()

	// Give the watcher time to set up inotify.
	time.Sleep(50 * time.Millisecond)

	// Simulate shell redirection: create empty file, then write content.
	f, err := os.Create(target)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // window where file is empty
	if _, err := f.WriteString("token-data"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	<-done

	if resultErr != nil {
		t.Fatalf("unexpected error: %v", resultErr)
	}
	if resultIdx != 0 {
		t.Errorf("got index %d, want 0", resultIdx)
	}
}

func TestNonemptyTimeout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "stays-empty.marker")

	// Create an empty file that stays empty.
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatalf("create empty marker: %v", err)
	}

	_, err := waitForFile([]string{target}, time.Now().Add(300*time.Millisecond), isNonEmpty)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if err.Error() != "timeout" {
		t.Errorf("got error %q, want %q", err.Error(), "timeout")
	}
}
