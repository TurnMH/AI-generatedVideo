FROM golang:1.23-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct \
	GOSUMDB=sum.golang.google.cn
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o image-service ./cmd

FROM alpine:3.19
RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.tuna.tsinghua.edu.cn/alpine|g' /etc/apk/repositories \
	&& apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/image-service .
EXPOSE 8005
CMD ["./image-service"]
