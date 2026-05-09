# autoVideo 项目公共 Makefile 配置
# 由 Makefile include 引用

COMPOSE_FILE      := infra/docker-compose.yml
COMPOSE_FULL_FILE := infra/docker-compose.full.yml
MIGRATIONS_DIR    := infra/migrations
PROTO_DIR         := proto
SERVICES_DIR      := services
TAG               ?= latest
ENV               ?= prod
