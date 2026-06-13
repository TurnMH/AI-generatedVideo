#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID=185
BASE_URL="${BASE_URL:-http://localhost:8000}"
JWT_SECRET="${JWT_SECRET:-autovideo-access-secret-change-in-prod}"
USER_ID="${USER_ID:-6}"
LOG_FILE="${LOG_FILE:-/tmp/extract-project-${PROJECT_ID}-ep2-48.log}"

exec > >(tee -a "$LOG_FILE") 2>&1

echo "=== [$(date '+%F %T')] incremental asset extraction for project ${PROJECT_ID} episodes 2-48 ==="

TOKEN=$(python3 - <<PY
import jwt, time, os
print(jwt.encode({
    "user_id": int(os.environ.get("USER_ID", "6")),
    "role": "service",
    "exp": int(time.time()) + 8 * 3600,
}, os.environ.get("JWT_SECRET", "autovideo-access-secret-change-in-prod"), algorithm="HS256"))
PY
)

auth_header() {
  echo "Authorization: Bearer ${TOKEN}"
}

api_post() {
  local path="$1"
  curl -sS -X POST "${BASE_URL}${path}" \
    -H "$(auth_header)" \
    -H "Content-Type: application/json" \
    -H "X-Autovideo-Skip-Storyboard-Trigger: true" \
    -d '{}' \
    -w "\nHTTP %{http_code}\n"
}

EPISODE_ROWS=()
while IFS='|' read -r eid enum title; do
  [[ -n "$eid" ]] && EPISODE_ROWS+=("${eid}|${enum}|${title}")
done < <(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -F '|' -c \
  "SELECT id, episode_number, COALESCE(title, '') FROM episodes WHERE project_id=${PROJECT_ID} AND episode_number >= 2 ORDER BY episode_number;")

if ((${#EPISODE_ROWS[@]} == 0)); then
  echo "no episodes found for project ${PROJECT_ID}"
  exit 1
fi

echo "[$(date '+%F %T')] dispatching ${#EPISODE_ROWS[@]} episode asset extractions"

for row in "${EPISODE_ROWS[@]}"; do
  eid="${row%%|*}"
  rest="${row#*|}"
  enum="${rest%%|*}"
  title="${rest#*|}"
  echo "[$(date '+%F %T')] episode ${enum} (${eid}) ${title}"
  resp=$(api_post "/api/v1/projects/${PROJECT_ID}/assets/extract-episode/${eid}" || true)
  echo "$resp" | tail -3
  sleep 0.2
done

wait_assets_done() {
  local tries=0
  while (( tries < 900 )); do
    local extracting settled total
    extracting=$(docker exec autovideo-postgres psql -U postgres -d character_db -t -A -c \
      "SELECT COUNT(*) FROM assets WHERE project_id=${PROJECT_ID} AND (name='__extracting__' OR status='extracting');" | tr -d '[:space:]')
    settled=$(docker exec autovideo-postgres psql -U postgres -d character_db -t -A -c \
      "SELECT COUNT(*) FROM assets WHERE project_id=${PROJECT_ID} AND name <> '__extracting__' AND status <> 'extracting';" | tr -d '[:space:]')
    total=$(docker exec autovideo-postgres psql -U postgres -d character_db -t -A -c \
      "SELECT COUNT(*) FROM assets WHERE project_id=${PROJECT_ID};" | tr -d '[:space:]')
    echo "[$(date '+%F %T')] assets total=${total:-0} settled=${settled:-0} extracting=${extracting:-0}"
    if [[ "${extracting:-0}" == "0" ]]; then
      return 0
    fi
    sleep 10
    tries=$((tries + 1))
  done
  echo "[$(date '+%F %T')] timed out waiting for asset extraction"
  return 1
}

wait_assets_done

echo "[$(date '+%F %T')] per-episode asset counts (top 12):"
docker exec autovideo-postgres psql -U postgres -d character_db -c \
  "SELECT unnest(episode_ids) AS episode_id, COUNT(*) AS assets
   FROM assets
   WHERE project_id=${PROJECT_ID} AND name <> '__extracting__'
   GROUP BY 1
   ORDER BY 1
   LIMIT 12;"

echo "[$(date '+%F %T')] done. log=${LOG_FILE}"
