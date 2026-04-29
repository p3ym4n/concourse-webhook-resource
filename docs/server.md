# Deploying the webhook server

The webhook server is a persistent HTTP service that receives inbound webhook calls and stores their payloads for the Concourse resource scripts to consume. It ships in the same Docker image as the resource itself.

---

## Environment variables

| Variable        | Default          | Description                                                                    |
|-----------------|------------------|--------------------------------------------------------------------------------|
| `PORT`          | `8080`           | Port the HTTP server listens on                                                |
| `STORAGE_PATH`  | `/data/webhooks` | Directory where payload JSON files are stored                                  |
| `WEBHOOK_TOKEN` | _(empty)_        | Secret token required on incoming webhooks. If empty, authentication is disabled |

---

## Docker

```bash
docker run -d \
  --name webhook-server \
  -p 8080:8080 \
  -v /data/webhooks:/data/webhooks \
  -e WEBHOOK_TOKEN=my-secret-token \
  ghcr.io/p3ym4n/concourse-webhook-resource:latest
```

## Docker Compose

```yaml
services:
  webhook-server:
    image: ghcr.io/p3ym4n/concourse-webhook-resource:latest
    ports:
      - "8080:8080"
    volumes:
      - webhook-data:/data/webhooks
    environment:
      WEBHOOK_TOKEN: my-secret-token
      PORT: 8080
      STORAGE_PATH: /data/webhooks

volumes:
  webhook-data:
```

## Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webhook-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: webhook-server
  template:
    metadata:
      labels:
        app: webhook-server
    spec:
      containers:
        - name: server
          image: ghcr.io/p3ym4n/concourse-webhook-resource:latest
          ports:
            - containerPort: 8080
          env:
            - name: WEBHOOK_TOKEN
              valueFrom:
                secretKeyRef:
                  name: webhook-secret
                  key: token
          volumeMounts:
            - name: storage
              mountPath: /data/webhooks
      volumes:
        - name: storage
          persistentVolumeClaim:
            claimName: webhook-storage-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: webhook-server
spec:
  selector:
    app: webhook-server
  ports:
    - port: 8080
      targetPort: 8080
```

---

## API reference

These endpoints are used internally by the resource scripts but can also be called directly for debugging.

| Method   | Path                       | Auth   | Description                        |
|----------|----------------------------|--------|------------------------------------|
| `GET`    | `/health`                  | None   | Returns `{"status":"ok"}`          |
| `POST`   | `/webhook`                 | Token  | Receive a new webhook payload      |
| `GET`    | `/api/payloads?after=<ts>` | Bearer | List payloads newer than timestamp |
| `GET`    | `/api/payloads/:id`        | Bearer | Fetch a specific payload by ID     |
| `DELETE` | `/api/payloads/:id`        | Bearer | Delete a payload by ID             |

The `after` parameter accepts an RFC3339Nano timestamp (e.g. `2024-01-15T10:30:00.000000000Z`).

Internal API endpoints (`/api/...`) authenticate via `Authorization: Bearer <token>` using the same token configured on the server.
