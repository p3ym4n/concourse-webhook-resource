# concourse-webhook-resource

A [Concourse CI](https://concourse-ci.org) resource that receives inbound webhook requests, stores their payloads, and triggers pipeline jobs — making every field in the webhook body available to downstream tasks.

> **Before you start:** this resource requires a webhook server to be running and reachable from your Concourse workers. See [docs/server.md](docs/server.md) for deployment instructions.

---

## How it works

The resource is made up of two components that ship as a single Docker image:

```
External system
      │
      │  POST /webhook
      ▼
┌─────────────────────┐    GET /api/payloads?after=…    ┌─────────────┐
│   webhook-server    │ ◄──────────────────────────────  │    check    │
│   (port 8080)       │                                  └─────────────┘
│   /data/webhooks/   │    GET /api/payloads/:id         ┌─────────────┐
│   (file storage)    │ ◄──────────────────────────────  │     in      │
└─────────────────────┘                                  └─────────────┘
```

1. **Webhook server** — a persistent HTTP service you deploy once. It receives `POST /webhook` calls from external systems and stores each payload on disk.
2. **`check`** — polls the server for new payloads and returns them as Concourse versions.
3. **`in`** (get step) — fetches the payload for a specific version and writes it to the build's working directory so tasks can consume it.
4. **`out`** (put step) — a no-op; exists only to satisfy the Concourse resource contract.

---

## Registering the resource type

```yaml
resource_types:
  - name: webhook
    type: registry-image
    source:
      repository: ghcr.io/p3ym4n/concourse-webhook-resource
      tag: latest
```

---

## Resource configuration (`source`)

```yaml
resources:
  - name: my-webhook
    type: webhook
    source:
      url: http://webhook-server:8080   # required — URL of the running webhook server
      token: ((webhook-token))          # optional — must match WEBHOOK_TOKEN on the server
      token_header: X-Webhook-Token     # optional — header name for incoming auth (default: X-Webhook-Token)
      cleanup: true                     # optional — delete payload after the get step fetches it (default: false)
```

| Field          | Required | Description                                                                    |
|----------------|----------|--------------------------------------------------------------------------------|
| `url`          | Yes      | Base URL of the webhook server                                                 |
| `token`        | No       | Shared secret; required when the server has `WEBHOOK_TOKEN` set                |
| `token_header` | No       | HTTP header name the server checks for the token (default: `X-Webhook-Token`) |
| `cleanup`      | No       | When `true`, the payload is deleted from the server after `in` fetches it      |

---

## Sending a webhook

Point your external system at `POST <server-url>/webhook`.

### Authentication

If `WEBHOOK_TOKEN` is set on the server, include the token in one of these ways:

```bash
# Via header (recommended)
curl -X POST http://webhook-server:8080/webhook \
  -H "X-Webhook-Token: my-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"branch": "main", "commit": "abc123", "author": "alice"}'

# Via query parameter
curl -X POST "http://webhook-server:8080/webhook?token=my-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"branch": "main", "commit": "abc123", "author": "alice"}'
```

The server responds with `202 Accepted` and the assigned payload ID:

```json
{ "id": "4b3f9a1c-..." }
```

### Non-JSON bodies

If the request body is not valid JSON, it is stored as-is under a `raw` key:

```json
{ "raw": "the raw body string" }
```

---

## Using the payload in a pipeline

After a `get` step on the resource, three formats are available in the build directory.

### Output file layout

```
my-webhook/
├── payload.json        ← full JSON body (pretty-printed)
├── vars.yml            ← flat YAML map for load_var
├── version             ← the webhook UUID
└── params/
    ├── branch          ← one file per top-level body field
    ├── commit
    └── author
```

### Option 1 — `load_var` (recommended for Concourse 7.8+)

Loads all payload fields as pipeline variables in one step:

```yaml
jobs:
  - name: on-webhook
    plan:
      - get: my-webhook
        trigger: true

      - load_var: payload
        file: my-webhook/vars.yml

      - task: deploy
        config:
          platform: linux
          image_resource:
            type: registry-image
            source: { repository: alpine }
          run:
            path: sh
            args:
              - -c
              - echo "Deploying branch $BRANCH at $COMMIT by $AUTHOR"
          params:
            BRANCH: ((payload.branch))
            COMMIT: ((payload.commit))
            AUTHOR: ((payload.author))
```

### Option 2 — Local variables via metadata

Each top-level payload field is available as a local variable immediately after the `get` step, without needing `load_var`:

```yaml
      - get: my-webhook
        trigger: true

      - task: deploy
        params:
          BRANCH: ((.:my-webhook.branch))
          COMMIT: ((.:my-webhook.commit))
          AUTHOR: ((.:my-webhook.author))
```

> The `((.:<resource-name>.<field>))` syntax is supported in Concourse 7.x+.

### Option 3 — Read files directly inside the task

```yaml
      - task: deploy
        config:
          inputs:
            - name: my-webhook
          run:
            path: sh
            args:
              - -c
              - |
                BRANCH=$(cat my-webhook/params/branch)
                COMMIT=$(cat my-webhook/params/commit)
                echo "Deploying $BRANCH @ $COMMIT"
```

---

## Type handling in `vars.yml`

Top-level fields in the webhook body are written with their natural YAML types:

| JSON type    | Example input         | `vars.yml` output       |
|--------------|-----------------------|-------------------------|
| String       | `"branch": "main"`    | `branch: "main"`        |
| Integer      | `"count": 3`          | `count: 3`              |
| Float        | `"ratio": 1.5`        | `ratio: 1.5`            |
| Boolean      | `"force": true`       | `force: true`           |
| Null         | `"tag": null`         | `tag: ~`                |
| Object/Array | `"meta": {"k": "v"}`  | `meta: "{\"k\":\"v\"}"` |

Nested objects and arrays are JSON-encoded into a quoted string so the `vars.yml` remains a flat map.

---

## Payload cleanup

By default (`cleanup: false`) payloads remain on the server after they are fetched. This is useful for debugging or when multiple pipelines consume the same webhook stream.

Set `cleanup: true` to have the `in` script delete the payload from the server immediately after fetching it, preventing it from triggering other pipeline runs.

---

## Complete pipeline example

```yaml
resource_types:
  - name: webhook
    type: registry-image
    source:
      repository: ghcr.io/p3ym4n/concourse-webhook-resource
      tag: latest

resources:
  - name: deploy-webhook
    type: webhook
    source:
      url: http://webhook-server:8080
      token: ((webhook-token))
      cleanup: true

jobs:
  - name: deploy-on-webhook
    plan:
      - get: deploy-webhook
        trigger: true

      - load_var: payload
        file: deploy-webhook/vars.yml

      - task: run-deploy
        config:
          platform: linux
          image_resource:
            type: registry-image
            source: { repository: alpine }
          inputs:
            - name: deploy-webhook
          run:
            path: sh
            args:
              - -c
              - |
                echo "Branch:  $BRANCH"
                echo "Commit:  $COMMIT"
                echo "Payload: $(cat deploy-webhook/payload.json)"
          params:
            BRANCH: ((payload.branch))
            COMMIT: ((payload.commit))
```
