#!/bin/bash
# 模型数据同步脚本：本地 model_db → 远程服务器
# 使用方式: ./sync_models.sh
# 说明: 本地新增/修改模型后运行，UPSERT 到远程，重复 model_key 自动去重（停用旧版）

set -e

REMOTE_USER="ubuntu"
REMOTE_HOST="118.89.83.96"
REMOTE_PASS="asdf#234@!"
SSHPASS="/opt/homebrew/bin/sshpass"
SSH_OPTS="-o ControlMaster=no -o StrictHostKeyChecking=no -o ServerAliveInterval=30"
TMP_SQL="/tmp/models_sync_$$.sql"
LOCAL_DB_CONTAINER="autovideo-postgres"
REMOTE_DB_CONTAINER="autovideo-postgres"

ssh_exec() {
  $SSHPASS -p "$REMOTE_PASS" ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST" "$@"
}

scp_put() {
  $SSHPASS -p "$REMOTE_PASS" scp $SSH_OPTS "$1" "$REMOTE_USER@$REMOTE_HOST:$2"
}

# ── 检查本地容器 ──────────────────────────────────────────────────────────────
if ! docker ps --format '{{.Names}}' | grep -q "^${LOCAL_DB_CONTAINER}$"; then
  echo "ERROR: 本地容器 ${LOCAL_DB_CONTAINER} 未运行，请先启动本地服务"
  exit 1
fi

echo "=== [1/4] 导出本地 models 表 ==="
docker exec "${LOCAL_DB_CONTAINER}" psql -U postgres -d model_db -t -A \
  -c "SELECT
  'INSERT INTO public.models (name, provider, type, api_endpoint, is_active, priority, cost_per_unit, unit, config, model_key, context_window, input_price, output_price, speed_rating, capability_tags, supports_consistency, consistency_method, video_mode, max_resolution, supported_ratios, is_default, api_key_ref, description, failure_reason, sort_order) VALUES (' ||
  quote_literal(name) || ',' ||
  quote_literal(provider) || ',' ||
  quote_literal(type) || ',' ||
  COALESCE(quote_literal(api_endpoint), 'NULL') || ',' ||
  is_active || ',' ||
  COALESCE(priority::text, '0') || ',' ||
  COALESCE(cost_per_unit::text, 'NULL') || ',' ||
  COALESCE(quote_literal(unit), 'NULL') || ',' ||
  COALESCE(quote_literal(config::text), 'NULL') || ',' ||
  COALESCE(quote_literal(model_key), 'NULL') || ',' ||
  COALESCE(context_window::text, 'NULL') || ',' ||
  COALESCE(input_price::text, 'NULL') || ',' ||
  COALESCE(output_price::text, 'NULL') || ',' ||
  COALESCE(quote_literal(speed_rating), 'NULL') || ',' ||
  COALESCE(quote_literal(capability_tags::text), '''{}''') || '::text[],' ||
  supports_consistency || ',' ||
  COALESCE(quote_literal(consistency_method), 'NULL') || ',' ||
  COALESCE(quote_literal(video_mode), 'NULL') || ',' ||
  COALESCE(quote_literal(max_resolution), 'NULL') || ',' ||
  COALESCE(quote_literal(supported_ratios::text), '''{}''') || '::text[],' ||
  is_default || ',' ||
  COALESCE(quote_literal(api_key_ref), 'NULL') || ',' ||
  COALESCE(quote_literal(description), 'NULL') || ',' ||
  COALESCE(quote_literal(failure_reason), 'NULL') || ',' ||
  COALESCE(sort_order::text, '0') ||
  ') ON CONFLICT (name, provider) DO UPDATE SET
    type              = EXCLUDED.type,
    api_endpoint      = EXCLUDED.api_endpoint,
    is_active         = EXCLUDED.is_active,
    priority          = EXCLUDED.priority,
    model_key         = EXCLUDED.model_key,
    context_window    = EXCLUDED.context_window,
    input_price       = EXCLUDED.input_price,
    output_price      = EXCLUDED.output_price,
    speed_rating      = EXCLUDED.speed_rating,
    capability_tags   = EXCLUDED.capability_tags,
    supports_consistency = EXCLUDED.supports_consistency,
    consistency_method = EXCLUDED.consistency_method,
    video_mode        = EXCLUDED.video_mode,
    max_resolution    = EXCLUDED.max_resolution,
    supported_ratios  = EXCLUDED.supported_ratios,
    is_default        = EXCLUDED.is_default,
    api_key_ref       = EXCLUDED.api_key_ref,
    description       = EXCLUDED.description,
    sort_order        = EXCLUDED.sort_order;'
  FROM models ORDER BY id;" 2>/dev/null > "${TMP_SQL}"

LOCAL_COUNT=$(wc -l < "${TMP_SQL}" | tr -d ' ')
echo "导出完成：${LOCAL_COUNT} 条模型记录"

# ── 传输到远程 ────────────────────────────────────────────────────────────────
echo ""
echo "=== [2/4] 传输到远程服务器 ==="
scp_put "${TMP_SQL}" "/tmp/models_sync.sql"
echo "传输完成"

# ── 执行 UPSERT ───────────────────────────────────────────────────────────────
echo ""
echo "=== [3/4] 在远程执行 UPSERT ==="
BEFORE=$(ssh_exec "sudo docker exec ${REMOTE_DB_CONTAINER} psql -U postgres -d model_db -t -A -c 'SELECT count(*) FROM models;'" 2>/dev/null | tr -d ' ')
echo "执行前远程模型总数：${BEFORE}"

ssh_exec "sudo docker exec -i ${REMOTE_DB_CONTAINER} psql -U postgres -d model_db < /tmp/models_sync.sql" 2>&1 | \
  grep -v '^INSERT\|^UPDATE' | head -10 || true

AFTER=$(ssh_exec "sudo docker exec ${REMOTE_DB_CONTAINER} psql -U postgres -d model_db -t -A -c 'SELECT count(*) FROM models;'" 2>/dev/null | tr -d ' ')
echo "执行后远程模型总数：${AFTER}（新增 $((AFTER - BEFORE)) 条）"

# ── 自动去重：停用同 model_key 中优先级较低（id 较小）的旧版本 ─────────────────
echo ""
echo "=== [4/4] 去重：停用重复 model_key 中的旧版记录 ==="
ssh_exec "
sudo docker exec ${REMOTE_DB_CONTAINER} psql -U postgres -d model_db -c \"
-- 对每个 model_key+type 组合，保留 id 最大的（最新同步版），停用其余
UPDATE models SET is_active = false
WHERE is_active = true
  AND model_key IS NOT NULL AND model_key <> ''
  AND id NOT IN (
    SELECT max(id)
    FROM models
    WHERE is_active = true AND model_key IS NOT NULL AND model_key <> ''
    GROUP BY model_key, type
  )
  AND (model_key, type) IN (
    SELECT model_key, type
    FROM models
    WHERE is_active = true AND model_key IS NOT NULL AND model_key <> ''
    GROUP BY model_key, type
    HAVING count(*) > 1
  );
SELECT 'deactivated_duplicates:' || count(*) FROM models WHERE is_active = false AND updated_at >= now() - interval '5 seconds';
\"
"

# ── 最终统计 ──────────────────────────────────────────────────────────────────
echo ""
echo "=== 同步结果 ==="
ssh_exec "sudo docker exec ${REMOTE_DB_CONTAINER} psql -U postgres -d model_db -c \
  'SELECT type, count(*) as total, sum(CASE WHEN is_active THEN 1 ELSE 0 END) as active FROM models GROUP BY type ORDER BY type;'"

# 清理临时文件
rm -f "${TMP_SQL}"
ssh_exec "rm -f /tmp/models_sync.sql" 2>/dev/null || true

echo ""
echo "=== 模型同步完成！$(date) ==="
echo "提示：在远程管理后台刷新模型列表，或重启 model-service 使配置生效"
