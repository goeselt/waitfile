# waitfile

Block until a file appears on disk, then exit with a code that says which one -- a lightweight barrier for CI steps and
containers that hand off work through the filesystem.

The usual way to wait for another step's artifact is a fixed `sleep`, which is either too slow or too flaky, or a
`until [ -f flag ]; do sleep 1; done` polling loop that adds latency and clutter. `waitfile` blocks on the real
filesystem event and continues the instant the file lands:

- **Race-free readiness.** Backed by Linux inotify, not a busy loop, so it wakes the moment the file is created and adds
  no polling latency or CPU spin.
- **First-one-wins across several files.** Wait on a set of paths -- say a success flag and a failure flag -- and the
  exit code tells you which appeared first, with no stdout parsing.
- **Waits for content, not just existence.** `-nonempty` holds until the file actually has data, so you never read a
  half-written marker that shell redirection created empty a moment before it was filled.
- **Tolerates a not-yet-created tree.** If a target's parent directory does not exist yet, `waitfile` polls for it, then
  watches -- useful when another container creates the directory.

Use `waitfile` when one step or container must wait for an artifact produced by another, and a fixed `sleep` is either
too slow or too fragile.

> [!NOTE]
>
> `waitfile` is Linux-only. It relies on inotify and is intended for CI runners and Linux hosts.

## Getting Started

### Install

Grab the latest binary from the [Releases](https://github.com/goeselt/waitfile/releases) page and put it on your
`PATH`, or install with Go:

```bash
go install github.com/goeselt/waitfile@latest
```

### Wait for a File

Block until a marker appears, then carry on:

```bash
waitfile /tmp/ready.flag && run-next-step
```

Race several files against a timeout; the exit code identifies the winner:

```bash
waitfile -t 60 /tmp/success.flag /tmp/failure.flag
case $? in
  0)  echo "success flag appeared" ;;
  10) echo "failure flag appeared" ;;
  1)  echo "timed out" ;;
esac
```

## Usage

```text
waitfile [-t <seconds>] [-nonempty] <path> [path...]
```

| Flag           | Description                                  |
| -------------- | -------------------------------------------- |
| `-t <seconds>` | Timeout in seconds (`0` = wait indefinitely) |
| `-nonempty`    | Wait until the file exists and has size > 0  |

The path that matched is printed to stdout. `waitfile` first checks whether any target already exists (fast path); if
not, it watches each target's parent directory via inotify and blocks until a matching event arrives or the timeout
expires.

## Exit Codes

| Code    | Meaning                                    |
| ------- | ------------------------------------------ |
| 0       | First path appeared                        |
| 10      | Second path appeared                       |
| 11      | Third path appeared                        |
| 12+     | Fourth path and beyond (`9 + index`)       |
| 1       | Timeout expired before any path appeared   |
| 2       | Usage error                                |

The match codes let a calling script tell which of several competing files appeared first without parsing stdout.

## Examples

```bash
# Wait indefinitely for a readiness marker
waitfile /tmp/ready.flag

# Wait up to 60s; distinguish success from failure by exit code
waitfile -t 60 /tmp/success.flag /tmp/failure.flag

# Wait until a token file is present and populated, not just created
waitfile -nonempty /run/secrets/token
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [LICENSE](LICENSE).
