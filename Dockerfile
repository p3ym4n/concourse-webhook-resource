FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY go.mod ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /opt/resource/check   ./cmd/check
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /opt/resource/in      ./cmd/in
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /opt/resource/out     ./cmd/out
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /opt/resource/server  ./cmd/server

# ── runtime image ────────────────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /opt/resource/ /opt/resource/
RUN chmod +x /opt/resource/*

VOLUME ["/data"]
EXPOSE 8080

# Default entrypoint runs the webhook server.
# The resource scripts (check/in/out) are invoked directly by Concourse.
ENTRYPOINT ["/opt/resource/server"]
