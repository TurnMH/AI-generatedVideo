# Docker 部署执行单

本次部署目标：

- 服务器：root@47.236.188.141
- 访问域名：10003.klyhtest.com
- 项目根目录：/home/autoVideo
- 服务目录：/home/autoVideo/autoVideo
- 前端静态目录：/home/autoVideo/web
- 环境文件：/home/autoVideo/autoVideo/infra/.env
- 后端镜像 tag：20260513

## 1. 本地准备

已完成：

- 后端镜像包：autoVideo/autovideo-backend-20260513.tar.gz

后端镜像必须按服务器架构构建：

```bash
cd /Users/hh/Desktop/Project/AI-generatedVideo/autoVideo
bash scripts/build.sh --tag=20260513 --platform=linux/amd64
docker save \
	autovideo/gateway:20260513 \
	autovideo/auth:20260513 \
	autovideo/project:20260513 \
	autovideo/script:20260513 \
	autovideo/character:20260513 \
	autovideo/image:20260513 \
	autovideo/video:20260513 \
	autovideo/task:20260513 \
	autovideo/model:20260513 \
	autovideo/storage:20260513 | gzip > autovideo-backend-20260513.tar.gz
```

本地前端静态导出命令：

```bash
cd /Users/hh/Desktop/Project/AI-generatedVideo/autoVideo/frontend
NEXT_PUBLIC_API_URL=/ API_PROXY_TARGET=http://127.0.0.1:8000 npm run build
```

## 2. 上传代码到服务器

```bash
cd /Users/hh/Desktop/Project/AI-generatedVideo
rsync -av --delete \
	--exclude .git \
	--exclude .DS_Store \
	--exclude node_modules \
	--exclude autoVideo/frontend/node_modules \
	--exclude autoVideo/frontend/.next \
	--exclude autoVideo/frontend/out \
	./ root@47.236.188.141:/home/autoVideo/
```

## 3. 上传后端镜像包

```bash
scp /Users/hh/Desktop/Project/AI-generatedVideo/autoVideo/autovideo-backend-20260513.tar.gz \
	root@47.236.188.141:/home/autoVideo/
```

## 4. 上传前端静态文件

```bash
ssh root@47.236.188.141 "mkdir -p /home/autoVideo/web"
rsync -av --delete \
	/Users/hh/Desktop/Project/AI-generatedVideo/autoVideo/frontend/out/ \
	root@47.236.188.141:/home/autoVideo/web/
```

## 5. 服务器加载后端镜像

```bash
ssh root@47.236.188.141
cd /home/autoVideo
gzip -dc autovideo-backend-20260513.tar.gz | docker load
docker images | grep autovideo
```

## 6. 服务器启动基础设施

```bash
cd /home/autoVideo/autoVideo
docker compose -f infra/docker-compose.yml --env-file infra/.env up -d
until docker exec autovideo-postgres pg_isready -U postgres -q 2>/dev/null; do sleep 3; done
```

## 7. 首次部署执行数据库迁移

```bash
cd /home/autoVideo/autoVideo
PG_PASS="$(grep POSTGRES_PASSWORD infra/.env | cut -d= -f2 | tr -d '\"' | tr -d ' ')"
for svc_dir in services/*/migrations; do
	[ -d "$svc_dir" ] || continue
	svc_name="$(echo "$svc_dir" | sed 's|.*/\([^/]*\)/migrations|\1|' | sed 's/-service//')"
	DB_URL="postgres://postgres:${PG_PASS}@localhost:5432/${svc_name}_db?sslmode=disable"
	migrate -path "$svc_dir" -database "$DB_URL" up || true
done
```

## 8. 启动后端服务

```bash
cd /home/autoVideo/autoVideo
AUTOVIDEO_TAG=20260513 docker compose -f infra/docker-compose.full.yml --env-file infra/.env up -d --remove-orphans
curl -sf http://127.0.0.1:8000/healthz && echo ok
```

## 9. Nginx 站点配置

服务器实际站点文件：

```bash
cat >/etc/nginx/conf.d/10003.klyhtest.com.conf <<'EOF'
server {
	listen 80;
	server_name 10003.klyhtest.com;

	location / {
		return 301 https://10003.klyhtest.com$request_uri;
	}
}

server {
	listen 443 ssl http2;
	server_name 10003.klyhtest.com;

	ssl_certificate     /etc/ssl/klyhtest.com/test.crt;
	ssl_certificate_key /etc/ssl/klyhtest.com/test.key;

	root /home/autoVideo/web;
	index index.html;
	etag off;
	gzip off;
	sendfile off;

	location /api/ {
		proxy_pass http://127.0.0.1:8000;
		proxy_http_version 1.1;
		proxy_set_header Host $host;
		proxy_set_header X-Real-IP $remote_addr;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_set_header X-Forwarded-Proto $scheme;
		proxy_read_timeout 300s;
	}

	location /ws {
		proxy_pass http://127.0.0.1:8000;
		proxy_http_version 1.1;
		proxy_set_header Upgrade $http_upgrade;
		proxy_set_header Connection "upgrade";
		proxy_set_header Host $host;
		proxy_set_header X-Real-IP $remote_addr;
		proxy_read_timeout 3600s;
	}

	location = /tools/video {
		return 302 /video-serial/quick;
	}

	location = /projects/new.txt {
		rewrite ^ /projects/new/index.txt last;
	}

	location = /video-serial/new.txt {
		rewrite ^ /video-serial/new/index.txt last;
	}

	location = /video-serial/quick.txt {
		rewrite ^ /video-serial/quick/index.txt last;
	}

	location /_next/static/ {
		access_log off;
		expires off;
		add_header Cache-Control "no-store, no-cache, must-revalidate, max-age=0" always;
		add_header Pragma "no-cache" always;
		add_header Expires "0" always;
		try_files $uri =404;
	}

	location ~ ^/projects/(?!__dynamic__/)[^/]+/characters/?$ {
		rewrite ^ /projects/__dynamic__/characters/ last;
	}

	location ~ ^/projects/(?!__dynamic__/)[^/]+/characters/index\.(txt|html)$ {
		rewrite ^ /projects/__dynamic__/characters/index.$1 last;
	}

	location ~ ^/projects/(?!__dynamic__/)[^/]+/generate/?$ {
		rewrite ^ /projects/__dynamic__/generate/ last;
	}

	location ~ ^/projects/(?!__dynamic__/)[^/]+/generate/index\.(txt|html)$ {
		rewrite ^ /projects/__dynamic__/generate/index.$1 last;
	}

	location ~ ^/projects/(?!__dynamic__/)[^/]+/storyboard/?$ {
		rewrite ^ /projects/__dynamic__/storyboard/ last;
	}

	location ~ ^/projects/(?!__dynamic__/)[^/]+/storyboard/index\.(txt|html)$ {
		rewrite ^ /projects/__dynamic__/storyboard/index.$1 last;
	}

	location ~ ^/projects/(?!(__dynamic__|new)/)[^/]+/index\.(txt|html)$ {
		rewrite ^ /projects/__dynamic__/index.$1 last;
	}

	location ~ ^/projects/(?!(__dynamic__|new|index\.txt|index\.html)(/|$))[^/]+/?$ {
		rewrite ^ /projects/__dynamic__/ last;
	}

	location ~ ^/video-serial/(?!__dynamic__/)[^/]+/generate/?$ {
		rewrite ^ /video-serial/__dynamic__/generate/ last;
	}

	location ~ ^/video-serial/(?!__dynamic__/)[^/]+/generate/index\.(txt|html)$ {
		rewrite ^ /video-serial/__dynamic__/generate/index.$1 last;
	}

	location ~ ^/video-serial/(?!(__dynamic__|new|quick)/)[^/]+/index\.(txt|html)$ {
		rewrite ^ /video-serial/__dynamic__/index.$1 last;
	}

	location ~ ^/video-serial/(?!(__dynamic__|new|quick|index\.txt|index\.html)(/|$))[^/]+/?$ {
		rewrite ^ /video-serial/__dynamic__/ last;
	}

	location / {
		add_header Cache-Control "no-store, no-cache, must-revalidate, max-age=0" always;
		add_header Pragma "no-cache" always;
		add_header Expires "0" always;
		try_files $uri $uri/ $uri.html /404.html;
	}
}
EOF
```

必须确认以下内容：

- root 改成 /home/autoVideo/web
- server_name 使用 10003.klyhtest.com
- 证书使用 /etc/ssl/klyhtest.com/test.crt 和 /etc/ssl/klyhtest.com/test.key
- /api 代理到 http://127.0.0.1:8000
- /ws 代理到 http://127.0.0.1:8000
- /projects 和 /video-serial 的 __dynamic__ rewrite 保留

重载 Nginx：

```bash
nginx -t
systemctl reload nginx
```

## 10. 验证

```bash
curl -I -H 'Host: 10003.klyhtest.com' http://127.0.0.1/
curl -k -I -H 'Host: 10003.klyhtest.com' https://127.0.0.1/
curl -k -I https://10003.klyhtest.com
docker compose -f /home/autoVideo/autoVideo/infra/docker-compose.full.yml --env-file /home/autoVideo/autoVideo/infra/.env ps
```

## 11. 故障排查

网关日志：

```bash
docker logs autovideo-gateway --tail 200
```

认证服务日志：

```bash
docker logs autovideo-auth --tail 200
```

项目服务日志：

```bash
docker logs autovideo-project --tail 200
```

图片服务日志：

```bash
docker logs autovideo-image --tail 200
```
