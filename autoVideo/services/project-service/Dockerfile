# ── Build stage ─────────────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
    && apk add --no-cache git ca-certificates tzdata

WORKDIR /app
ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="${TARGETARCH:-$(go env GOARCH)}" \
    go build -ldflags="-w -s" -o /app/project-service ./cmd

FROM alpine:3.20 AS healthcheck-tools

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
    && apk add --no-cache busybox-static

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/project-service /project-service
COPY --from=healthcheck-tools /bin/busybox.static /bin/busybox

# HTTP port
EXPOSE 8002
# gRPC port
EXPOSE 9002

ENTRYPOINT ["/project-service"]
