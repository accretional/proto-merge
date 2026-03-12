# merge

Protobuf source code processor, designed to be bundled into other services as a Go package.

## What it does

**merge** scans GitHub organizations/users for `.proto` files, downloads them, and produces three output formats:

### Output Directories

| Directory | Description |
|---|---|
| `proto-download/` | Raw `.proto` files downloaded from GitHub, preserving original repo/path structure |
| `proto-bundle/` | All discovered protobuf types merged into a single `.proto` file |
| `proto-split/` | Each gRPC message and service in its own file, organized by package directory |

### Core Capabilities

- **GitHub Scanner** (`github/` package): Uses the GitHub API to scan all repositories in an organization for `.proto` files, collecting their contents
- **Downloader**: Fetches and saves all discovered `.proto` files to `proto-download/`
- **Bundler**: Merges all protobuf types (messages, services, enums, RPCs) into a single consolidated `.proto` file in `proto-bundle/`
- **Splitter**: Decomposes proto definitions so each message and service gets its own file, organized into per-package directories under `proto-split/`

## Usage

### CLI

```bash
go run ./cmd/ --org accretional
```

### As a package

```go
import "github.com/accretional/merge/github"
import "github.com/accretional/merge/proto"
```

## Development

```bash
# Run
go run ./cmd/

# Test
bash tests.sh
```

## Validation

This project uses `.proto` files from [github.com/accretional](https://github.com/accretional) (especially `runrpc`) as the primary test corpus.
