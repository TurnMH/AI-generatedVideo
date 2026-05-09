FROM golang:1.22-alpine AS builder

WORKDIR /app

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o video-service ./cmd/main.go

# ── Runtime image ──────────────────────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates: TLS; tzdata: timezone support; ffmpeg: video processing
RUN apk add --no-cache ca-certificates tzdata ffmpeg

WORKDIR /app

COPY --from=builder /app/video-service .

# Default temp directory for FFmpeg working files
RUN mkdir -p /tmp/video-service

EXPOSE 8006

ENV GIN_MODE=release

CMD ["./video-service"]
