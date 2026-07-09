// waitfile blocks until one or more specified files appear on disk, then exits.
//
// Usage: waitfile [-t <seconds>] [-nonempty] <path> [path...]
//
// Exit codes:
//
//	0  - first path appeared
//	10 - second path appeared
//	11 - third path appeared
//	12 - fourth path appeared (and so on)
//	1  - timeout expired before any path appeared
//	2  - usage error
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("waitfile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.Int("t", 0, "timeout in seconds (0 = wait indefinitely)")
	nonempty := fs.Bool("nonempty", false, "wait until the file exists and is non-empty")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "Usage: waitfile [-t <seconds>] [-nonempty] <path> [path...]\n\n")
		_, _ = fmt.Fprintf(stderr, "Block until one of the specified files appears on disk.\n\n")
		_, _ = fmt.Fprintf(stderr, "Exit codes:\n")
		_, _ = fmt.Fprintf(stderr, "  0   first path appeared\n")
		_, _ = fmt.Fprintf(stderr, "  10  second path appeared\n")
		_, _ = fmt.Fprintf(stderr, "  11  third path appeared (and so on)\n")
		_, _ = fmt.Fprintf(stderr, "  1   timeout\n")
		_, _ = fmt.Fprintf(stderr, "  2   usage error\n\n")
		_, _ = fmt.Fprintf(stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	paths := fs.Args()
	if len(paths) == 0 {
		fs.Usage()
		return 2
	}

	var deadline time.Time
	if *timeout > 0 {
		deadline = time.Now().Add(time.Duration(*timeout) * time.Second)
	}

	var contentOK func(string) bool
	if *nonempty {
		contentOK = isNonEmpty
	}

	idx, err := waitForFile(paths, deadline, contentOK)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "waitfile: %s\n", err)
		return 1
	}

	_, _ = fmt.Fprintln(stdout, paths[idx])
	return exitCode(idx)
}

// exitCode maps a matched path index to the documented exit code.
// Index 0 to 0, index 1 to 10, index 2 to 11, index 3 to 12, ...
func exitCode(index int) int {
	if index == 0 {
		return 0
	}
	return 9 + index
}

// isNonEmpty returns true when the file at path exists and has size > 0.
func isNonEmpty(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}
