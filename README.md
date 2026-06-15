# Flomation Executor

> Runtime engine — executes a flo's node graph and reports results.

## Overview

The Flomation Executor is the runtime engine for the Flomation Automate platform. It
takes a flow definition file (a JSON graph of nodes and edges), resolves variable
substitutions from upstream outputs, environment properties, and secrets, then executes
each node's action in dependency order. Results are written to a `state.json` file on
completion.

The executor is a single-shot CLI binary. In production it is invoked as a subprocess by
the Runner, which
supplies the flow definition and reports the resulting state back to the
API. It can also be run
directly against a local flow file.

## Prerequisites

- Go 1.26.1+
- Docker (optional, for containerised execution)

## Installation

**Build from source (all platforms):**

```sh
make build
```

This produces binaries under `dist/` for linux/amd64, linux/arm64, linux/arm,
darwin/amd64, darwin/arm64, and windows/amd64.

**Install locally:**

```sh
make install
```

Installs the binary as `flomation-executor` in your `$GOPATH/bin`.

## Configuration

The executor is configured entirely through command-line flags:

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `-path` | Path to the flow JSON file | Yes | — |
| `-entry` | Entry node ID to begin execution | No | First `trigger/manual` node |
| `-id` | Execution ID | No | Random UUID |
| `-flow` | Flow ID | No | — |
| `-api` | Flomation API URL | No | `https://api.flomation.app` |
| `-environment` | Environment ID to execute within | No | `default` |
| `-runner` | Runner ID | No | `local` |
| `-output` | Log output format (`default` or `json`) | No | `default` |
| `-user` | Username for environment auth | No | — |
| `-password` | Password for environment auth | No | — |
| `-token` | Bearer token for environment auth | No | — |
| `-key` | RSA private key file for certificate auth | No | — |
| `-identity` | Identity service URL | No | `https://id.flomation.app` |
| `-debug` | Enable debug logging | No | `false` |
| `-version` | Print version and exit | No | `false` |
| `-manifest` | Path to export the action manifest JSON | No | — |

## Usage

**Execute a flow:**

```sh
flomation-executor -path flow.json
```

**Execute with a specific entry node and environment:**

```sh
flomation-executor \
  -path flow.json \
  -entry node-abc-123 \
  -environment production \
  -token eyJhbGciOi...
```

**Export the action manifest:**

```sh
flomation-executor -manifest manifest.json
```

**Flow definition format:**

A flow is a JSON file containing `nodes` and `edges`:

```json
{
  "nodes": [
    {
      "id": "trigger-1",
      "type": "trigger/manual",
      "data": {
        "config": {
          "id": "trigger-1",
          "type": 1,
          "plugin": "trigger/manual",
          "inputs": [],
          "outputs": []
        }
      }
    },
    {
      "id": "add-1",
      "type": "arithmetic/addition",
      "data": {
        "config": {
          "id": "add-1",
          "type": 2,
          "plugin": "arithmetic/addition",
          "inputs": [
            { "name": "a", "type": "integer", "value": 10 },
            { "name": "b", "type": "integer", "value": 20 }
          ],
          "outputs": [
            { "name": "result", "type": "integer" }
          ]
        }
      }
    }
  ],
  "edges": [
    {
      "id": "e1",
      "source": "trigger-1",
      "target": "add-1"
    }
  ]
}
```

On completion, the executor writes a `state.json` file:

```json
{
  "id": "execution-uuid",
  "flow_id": "flow-id",
  "status": 0,
  "duration": 42,
  "billing_duration": 42,
  "outputs": { "result": 30 }
}
```

A `status` of `0` indicates success; `1` indicates an error.

## Actions

| Action | Description |
|--------|-------------|
| `trigger/manual` | Manual trigger — entry point for flow execution |
| `output/set` | Set a named output value on the flow |
| `conditional/if` | Conditional branching (true/false paths) |
| `arithmetic/addition` | Add two numbers |
| `arithmetic/subtraction` | Subtract two numbers |
| `arithmetic/multiplication` | Multiply two numbers |
| `arithmetic/division` | Divide two numbers |
| `git/clone` | Clone a Git repository |
| `git/checkout` | Checkout a branch or commit |
| `git/add` | Stage files |
| `git/commit` | Create a commit |
| `git/push` | Push to remote |
| `git/pull` | Pull from remote |
| `git/branch` | Create or list branches |
| `git/tag` | Create a tag |
| `git/status` | Get repository status |
| `aws/s3/list` | List S3 buckets or objects |
| `aws/s3/get` | Download an object from S3 |
| `aws/s3/put` | Upload an object to S3 |
| `aws/s3/delete` | Delete an object from S3 |
| `aws/ec2/describe` | Describe EC2 instances |
| `sql/query` | Execute a SQL query (PostgreSQL) |
| `common/smtp` | Send an email via SMTP |
| `security/antivirus` | Scan a file with ClamAV |

## Variable Substitution

Node inputs support variable substitution using `${...}` syntax:

| Pattern | Source | Example |
|---------|--------|---------|
| `${env.name}` | Environment property | `${env.database_host}` |
| `${secret.name}` | Environment secret | `${secret.db_password}` |
| `${output_name}` | Upstream node output | `${result}` |

## Development

```sh
# Run tests
make test

# Lint (runs goimports, golangci-lint, go vet, gosec, govulncheck)
make lint

# Regenerate the action manifest (writes internal/assets/manifest/manifest.json)
make manifest
```

## Docker

Base image: `dhi.io/alpine-base:3.23-alpine3.23-dev`. Runs as a non-root `flomation` user.

```sh
docker build --build-arg BINARY_FILE=dist/flomation-executor-amd64-linux-1.0.local -t flomation-executor .
docker run flomation-executor -path /path/to/flow.json
```

## Project Structure

```
.
├── cmd/
│   ├── main.go                  # CLI entry point
│   └── manifest/                # Manifest generator
├── flow.go                      # Core engine (Flow, Node, Edge, execution)
├── actions/
│   ├── actions.go               # Action registry
│   ├── arithmetic/              # Addition, subtraction, multiplication, division
│   ├── aws/                     # EC2 describe, S3 get/put/delete/list
│   ├── common/smtp/             # SMTP email
│   ├── conditional/if/          # Conditional branching
│   ├── git/                     # Clone, checkout, add, commit, push, pull, branch, tag, status
│   ├── output/set/              # Set flow output
│   ├── security/antivirus/      # ClamAV scan
│   ├── sql/query/               # PostgreSQL query
│   └── trigger/manual/          # Manual trigger
├── internal/
│   ├── assets/manifest/         # Embedded action manifest
│   ├── environment/             # Environment properties & secrets via API
│   └── version/                 # Build version info
├── Dockerfile
├── Makefile
└── go.mod
```

## Licence

MIT — Flomation LTD. See [LICENCE.md](LICENCE.md).
