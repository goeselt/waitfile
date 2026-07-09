# Contributing to waitfile

## Design

| File         | Responsibility                                                                                                                         |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| `main.go`    | CLI entry point: flag parsing, timeout deadline, content-check selection, exit-code mapping, matched-path output.                      |
| `watcher.go` | `waitForFile` inotify watcher: parent-directory watches, fast-path and race re-check, `-nonempty` re-check, missing-directory polling. |

`waitForFile` checks existence first (fast path), then adds an inotify watch on each target's parent directory and
re-checks once more to close the create-before-watch race. With `-nonempty` it also watches `IN_MODIFY` /
`IN_CLOSE_WRITE` so it can re-check file size after a writer finishes. A parent directory that does not exist yet is
polled every 250 ms until it appears or the deadline passes; each round re-checks all targets so paths in existing
directories are not starved while one directory is missing. inotify makes the tool and its tests Linux-only.

## Development Setup

Go 1.24 or later. One external dependency (`golang.org/x/sys`).

```bash
git clone https://github.com/goeselt/waitfile.git
cd waitfile
make build
```

## Local Verification

Run the same checks used by CI:

```bash
make check
```

Lint:

```bash
docker pull ghcr.io/goeselt/pedant:latest
docker run --rm -v "$(pwd):/work" ghcr.io/goeselt/pedant:latest
```

## Submitting Changes

Commit messages and PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/). The release
pipeline uses the PR title to determine the next version.
