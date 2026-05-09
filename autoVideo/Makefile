.PHONY: infra-up infra-down infra-logs infra-full-up infra-full-down \
        migrate-all proto-gen lint test build-all dev-all \
        gateway gateway-build gateway-run \
        build push deploy

include config.mk

infra-up:
	docker compose -f $(COMPOSE_FILE) up -d

infra-down:
	docker compose -f $(COMPOSE_FILE) down

infra-logs:
	docker compose -f $(COMPOSE_FILE) logs -f

# 全量启动（基础设施 + 所有服务 + Gateway）
infra-full-up:
	docker compose -f $(COMPOSE_FULL_FILE) --env-file infra/.env up -d

infra-full-down:
	docker compose -f $(COMPOSE_FULL_FILE) down

migrate-all:
	@for svc_dir in $(SERVICES_DIR)/*/migrations; do \
		svc_name=$$(basename $$(dirname $$svc_dir) | sed 's/-service//'); \
		echo "==> Migrating $$svc_name"; \
		migrate -path $$svc_dir -database "postgres://postgres:postgres@localhost:5432/$${svc_name}_db?sslmode=disable" up; \
	done

proto-gen:
	@for proto_file in $(PROTO_DIR)/*.proto; do \
		echo "==> Generating $$proto_file"; \
		protoc \
			--go_out=. \
			--go_opt=paths=source_relative \
			--go-grpc_out=. \
			--go-grpc_opt=paths=source_relative \
			$$proto_file; \
	done

lint:
	golangci-lint run ./...

test:
	go test ./... -v -race -coverprofile=coverage.out

build-all:
	@for svc in $(SERVICES_DIR)/*/; do \
		svc_name=$$(basename $$svc); \
		echo "==> Building $$svc_name"; \
		go build -o bin/$$svc_name ./$(SERVICES_DIR)/$$svc_name/cmd/...; \
	done

dev-all:
	@if command -v goreman > /dev/null 2>&1; then \
		goreman start; \
	elif command -v air > /dev/null 2>&1; then \
		air; \
	else \
		for svc in $(SERVICES_DIR)/*/; do \
			svc_name=$$(basename $$svc); \
			echo "==> Starting $$svc_name"; \
			go run ./$(SERVICES_DIR)/$$svc_name/cmd/... & \
		done; \
		wait; \
	fi

# 本地一键开发启动（含基础设施+迁移）
dev:
	bash scripts/dev.sh

# gateway-service：统一入口，监听 :8000
gateway-build:
	go build -o services/gateway-service/gateway-service ./services/gateway-service/cmd/...

gateway-run: gateway-build
	cd services/gateway-service && ./gateway-service -config "$${AUTOVIDEO_GATEWAY_CONFIG_FILE:-config.local.yaml}"

gateway:
	cd services/gateway-service && go run ./cmd/main.go -config "$${AUTOVIDEO_GATEWAY_CONFIG_FILE:-config.local.yaml}"

# 构建所有 Docker 镜像
build:
	bash scripts/build.sh --env=$(ENV) --tag=$(TAG)

# 构建并推送镜像
push:
	bash scripts/build.sh --env=$(ENV) --tag=$(TAG) --push

# 线上部署
deploy:
	bash scripts/deploy.sh --env=$(ENV) --tag=$(TAG)

# 生成 .env 文件模板
env-init:
	@if [ ! -f infra/.env ]; then \
		cp infra/.env.example infra/.env; \
		echo "已创建 infra/.env，请填入真实配置"; \
	else \
		echo "infra/.env 已存在"; \
	fi
