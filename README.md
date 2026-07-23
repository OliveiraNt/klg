# klg — kubectl log formatter

A small Go CLI that reads a `kubectl` log stream from stdin and prints a
formatted version for better readability: timestamp, colored level, message
and aligned structured fields.

It automatically detects:

- **JSON** (`{"time":"...","level":"info","msg":"..."}`)
- **logfmt** (`ts=... level=info msg="..."`)
- **Free text** (uses heuristics to detect the level)
- RFC3339 timestamp prefix from `kubectl logs --timestamps`

## Installation

### Script (Linux / macOS / Git Bash on Windows)

Installs the latest binary, automatically detecting OS and architecture:

```sh
curl -fsSL https://raw.githubusercontent.com/OliveiraNt/klg/main/install.sh | sh
```

Useful options:

```sh
# specific version
curl -fsSL https://raw.githubusercontent.com/OliveiraNt/klg/main/install.sh | sh -s -- v0.1.0

# custom install directory
curl -fsSL https://raw.githubusercontent.com/OliveiraNt/klg/main/install.sh | sh -s -- -b $HOME/.local/bin
```

The script tries to install into `/usr/local/bin` (using `sudo` when needed) and falls back to `$HOME/.local/bin`. It verifies the checksum against the release's `checksums.txt`.

### Via `go install`

```sh
go install github.com/OliveiraNt/klg@latest
```

### Manual download

Prebuilt binaries for Linux, macOS and Windows (amd64/arm64) are available on the [Releases](https://github.com/OliveiraNt/klg/releases) page.

## Local build

```sh
go build -o klg .
```

To embed version information into a local build:

```sh
go build -ldflags "-X main.version=dev -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o klg .
```

Release builds embed these values automatically via GoReleaser.

## Release

Releases are generated automatically by [GoReleaser](https://goreleaser.com) via GitHub Actions when a `vX.Y.Z` tag is published:

```sh
git tag v0.1.0
git push origin v0.1.0
```

## Usage

```sh
kubectl logs -f my-pod | klg
kubectl logs -f my-pod --timestamps | klg --level=warn
kubectl logs my-pod | klg --json-pretty
kubectl logs my-pod | klg --raw --no-color
```

Show the installed version:

```sh
klg --version
# or
klg version
```

### Flags

| Flag            | Description                                                        |
|-----------------|--------------------------------------------------------------------|
| `--no-color`    | disable ANSI colors (auto when stdout is not a tty)                |
| `--raw`         | append the original line below the formatted one                   |
| `--level`       | filter by minimum level (`debug`, `info`, `warn`, `error`)         |
| `--time`        | Go-style time layout (default `15:04:05`)                          |
| `--json-pretty` | expand JSON values (objects/arrays) as an indented, colored block  |
| `--version`     | print version information and exit                                 |

## Style

Rendering uses [lipgloss](https://github.com/charmbracelet/lipgloss):
levels are rendered as *badges* with a background color per severity
(`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`) and fields are colored
as `key=value`. With `--json-pretty`, fields whose value is a JSON
object/array are printed on multiple lines with colored syntax.

## Test data

The [`testdata/`](testdata) directory contains dummy log files so you can try
out the CLI without a cluster:

```sh
cat testdata/json.log              | ./klg --json-pretty
cat testdata/logfmt.log            | ./klg
cat testdata/plain.log             | ./klg
cat testdata/kubectl-timestamps.log| ./klg
cat testdata/mixed.log             | ./klg --json-pretty
```

| File                        | Format                                    |
|-----------------------------|-------------------------------------------|
| `json.log`                  | structured JSON logs                      |
| `logfmt.log`                | `key=value` style logs (logfmt)           |
| `plain.log`                 | free text with inferred levels            |
| `kubectl-timestamps.log`    | output of `kubectl logs --timestamps`     |
| `mixed.log`                 | a mix of all formats above                |

## Layout

```
klg/
├── main.go
├── go.mod
└── internal/
    ├── parser/     # format detection and input normalization
    └── formatter/  # colored rendering
```
