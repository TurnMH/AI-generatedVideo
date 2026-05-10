# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
    && apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /storage-service ./cmd

# ── Runtime stage ───────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /storage-service .

EXPOSE 8008 9008

ENTRYPOINT ["/app/storage-service"]
