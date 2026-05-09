FROM golang:1.22-alpine AS builder

WORKDIR /app

# 安装必要的系统依赖
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o pipeline-service ./cmd/main.go

# ─── 运行镜像 ─────────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/pipeline-service .

EXPOSE 8010

CMD ["./pipeline-service"]
