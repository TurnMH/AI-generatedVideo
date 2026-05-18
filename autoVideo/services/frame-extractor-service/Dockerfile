FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn

WORKDIR /app
ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="${TARGETARCH:-$(go env GOARCH)}" go build -trimpath -ldflags="-s -w" -o frame-extractor-service ./cmd

FROM alpine:3.19

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata ffmpeg wget

WORKDIR /app

COPY --from=builder /app/frame-extractor-service .

EXPOSE 8010

ENV GIN_MODE=release

CMD ["./frame-extractor-service"]