FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct \
	GOSUMDB=sum.golang.google.cn

WORKDIR /app
ARG TARGETOS
ARG TARGETARCH

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="${TARGETARCH:-$(go env GOARCH)}" go build -trimpath -ldflags="-s -w" -o video-service ./cmd

# ── Runtime image ──────────────────────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates: TLS; tzdata: timezone support; ffmpeg: video processing;
# python3 + edge-tts: dubbing/subtitle generation via Microsoft Edge TTS CLI
RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
	&& apk add --no-cache ca-certificates tzdata ffmpeg python3 py3-pip \
	&& pip3 install --no-cache-dir --break-system-packages 'edge-tts>=6.1.9' \
	&& edge-tts --version

WORKDIR /app

COPY --from=builder /app/video-service .

# Default temp directory for FFmpeg working files
RUN mkdir -p /tmp/video-service

EXPOSE 8006

ENV GIN_MODE=release

CMD ["./video-service"]
