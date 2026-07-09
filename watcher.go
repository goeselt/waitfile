package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// errTimeout reports that the deadline expired before any path matched.
// run maps it to the documented timeout exit code.
var errTimeout = errors.New("timeout")

// waitForFile blocks until one of the target paths exists on disk and satisfies
// the optional content check.  When contentOK is nil the file only needs to
// exist; when set, the file must also pass the check (e.g. non-empty).
// It returns the index of the first path that matched.
// A zero-value deadline means wait indefinitely.
func waitForFile(paths []string, deadline time.Time, contentOK func(string) bool) (int, error) {
	// Fast path: check if any file already exists (and satisfies content check).
	if idx, ok := checkMatch(paths, contentOK); ok {
		return idx, nil
	}

	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		return -1, fmt.Errorf("inotify_init: %w", err)
	}
	defer func() { _ = unix.Close(fd) }()

	// Watch each parent directory for file creation and content changes.
	type watchEntry struct {
		wd   int32
		base string
		idx  int
	}
	var watches []watchEntry

	// When a content check is active we also need IN_MODIFY and IN_CLOSE_WRITE
	// so we can re-check after the writer finishes populating the file.
	watchMask := uint32(unix.IN_CREATE | unix.IN_MOVED_TO)
	if contentOK != nil {
		watchMask |= unix.IN_MODIFY | unix.IN_CLOSE_WRITE
	}

	dirWds := make(map[string]int32) // dedup watches on the same directory

	for i, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return -1, fmt.Errorf("abs(%q): %w", p, err)
		}
		dir := filepath.Dir(abs)
		base := filepath.Base(abs)

		wd, exists := dirWds[dir]
		if !exists {
			// The parent directory may not exist yet (e.g. another container creates
			// it).  Poll until it appears or the deadline expires, then add the watch.
			wd32, err := unix.InotifyAddWatch(fd, dir, watchMask)
			for err != nil && errors.Is(err, unix.ENOENT) {
				// Watches for the remaining paths are not established yet, so
				// re-check all targets each round to avoid starving them.
				if idx, ok := checkMatch(paths, contentOK); ok {
					return idx, nil
				}
				if !deadline.IsZero() && time.Now().After(deadline) {
					return -1, errTimeout
				}
				time.Sleep(250 * time.Millisecond)
				wd32, err = unix.InotifyAddWatch(fd, dir, watchMask)
			}
			if err != nil {
				return -1, fmt.Errorf("inotify_add_watch(%q): %w", dir, err)
			}
			wd = int32(wd32) //nolint:gosec // G115: inotify watch descriptor fits in int32
			dirWds[dir] = wd
		}
		watches = append(watches, watchEntry{wd: wd, base: base, idx: i})
	}

	// Re-check after watches are established (close the race window).
	if idx, ok := checkMatch(paths, contentOK); ok {
		return idx, nil
	}

	// Poll loop on the inotify fd with timeouts.
	buf := make([]byte, 4096)
	for {
		timeoutMs := -1 // block indefinitely
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return -1, errTimeout
			}
			timeoutMs = int(remaining.Milliseconds())
			if timeoutMs <= 0 {
				timeoutMs = 1
			}
		}

		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}} //nolint:gosec // G115: fd fits in int32
		n, err := unix.Poll(fds, timeoutMs)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return -1, fmt.Errorf("poll: %w", err)
		}
		if n == 0 {
			return -1, errTimeout
		}

		nBytes, err := unix.Read(fd, buf)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return -1, fmt.Errorf("read inotify: %w", err)
		}

		// Parse inotify events.
		offset := 0
		for offset < nBytes {
			if offset+unix.SizeofInotifyEvent > nBytes {
				break
			}
			ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset])) //nolint:gosec // G103: required for inotify event parsing
			nameLen := int(ev.Len)
			var name string
			if nameLen > 0 {
				nameBytes := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+nameLen]
				// Trim null bytes.
				for i, b := range nameBytes {
					if b == 0 {
						nameBytes = nameBytes[:i]
						break
					}
				}
				name = string(nameBytes)
			}
			offset += unix.SizeofInotifyEvent + nameLen

			for _, w := range watches {
				if ev.Wd == w.wd && name == w.base {
					if contentOK == nil || contentOK(paths[w.idx]) {
						return w.idx, nil
					}
				}
			}
		}
	}
}

// checkMatch returns the index of the first path that exists and satisfies
// contentOK.  When contentOK is nil, only existence is checked.
func checkMatch(paths []string, contentOK func(string) bool) (int, bool) {
	for i, p := range paths {
		if _, err := os.Stat(p); err == nil {
			if contentOK == nil || contentOK(p) {
				return i, true
			}
		}
	}
	return -1, false
}
