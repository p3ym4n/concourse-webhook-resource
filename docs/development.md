# Development

## Prerequisites

- Go 1.26+
- Docker (for building the image)

---

## Project structure

```
concourse-webhook-resource/
├── cmd/
│   ├── check/      ← Concourse check script
│   ├── in/         ← Concourse in script
│   ├── out/        ← Concourse out script (no-op)
│   └── server/     ← Webhook HTTP server
├── internal/
│   ├── models/     ← Shared types (Source, Version, Metadata, WebhookPayload)
│   ├── server/     ← HTTP handlers and routing
│   └── storage/    ← File-backed payload storage
├── docs/
├── Dockerfile
└── go.mod
```

No external Go dependencies — only the standard library.

---

## Running tests

```bash
go test ./...
```

## Building binaries

```bash
go build -o bin/check   ./cmd/check
go build -o bin/in      ./cmd/in
go build -o bin/out     ./cmd/out
go build -o bin/server  ./cmd/server
```

## Building the Docker image

```bash
docker build -t concourse-webhook-resource .
```

---

## Running the server locally

```bash
WEBHOOK_TOKEN=dev-token STORAGE_PATH=/tmp/webhooks PORT=8080 go run ./cmd/server
```

Send a test webhook:

```bash
curl -X POST http://localhost:8080/webhook \
  -H "X-Webhook-Token: dev-token" \
  -H "Content-Type: application/json" \
  -d '{"branch": "main", "commit": "abc123"}'
```

Inspect stored payloads:

```bash
curl -H "Authorization: Bearer dev-token" http://localhost:8080/api/payloads
```

---

## Simulating the resource scripts

The resource scripts read JSON from stdin and write JSON to stdout, matching the [Concourse resource protocol](https://concourse-ci.org/implementing-resource-types.html).

**check** — returns new versions since the given version:

```bash
echo '{"source":{"url":"http://localhost:8080","token":"dev-token"},"version":null}' \
  | go run ./cmd/check
```

**in** — fetches a payload and writes files to a destination directory:

```bash
echo '{"source":{"url":"http://localhost:8080","token":"dev-token"},"version":{"id":"<id>","timestamp":"<ts>"}}' \
  | go run ./cmd/in /tmp/out

ls /tmp/out
# payload.json  vars.yml  version  params/
```

**out** — no-op, returns a timestamp version:

```bash
echo '{"source":{"url":"http://localhost:8080"},"params":{}}' \
  | go run ./cmd/out
```
