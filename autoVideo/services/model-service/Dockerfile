# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary (static, no CGO required for pure-Go postgres driver)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /model-service ./cmd/main.go

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM scratch

# CA certs + timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Binary
COPY --from=builder /model-service /model-service

# Migrations (optional, can be applied externally)
COPY --from=builder /app/migrations /migrations

# HTTP port  : 8007
# gRPC port  : 9007
EXPOSE 8007 9007

ENTRYPOINT ["/model-service"]
