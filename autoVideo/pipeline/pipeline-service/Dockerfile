FROM golang:1.23-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct \
	GOSUMDB=sum.golang.google.cn

WORKDIR /app

# 安装必要的系统依赖
RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
	&& apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o pipeline-service ./cmd

# ─── 运行镜像 ─────────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
	&& apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/pipeline-service .

EXPOSE 8010

CMD ["./pipeline-service"]
