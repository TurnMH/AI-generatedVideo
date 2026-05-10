# ── build stage ──────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct \
	GOSUMDB=sum.golang.google.cn

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
	&& apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o task-service ./cmd

# ── runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
	&& apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/task-service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8010 9010

ENTRYPOINT ["./task-service"]
