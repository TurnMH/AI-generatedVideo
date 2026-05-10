# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
    && apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary (static, no CGO required for pure-Go postgres driver)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /model-service ./cmd

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
