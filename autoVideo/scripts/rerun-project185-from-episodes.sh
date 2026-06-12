#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID=185
BASE_URL="${BASE_URL:-http://localhost:8002}"
JWT_SECRET="${JWT_SECRET:-autovideo-access-secret-change-in-prod}"
USER_ID="${USER_ID:-6}"
PARALLEL="${PARALLEL:-3}"
LOG_FILE="${LOG_FILE:-/tmp/rerun-project-${PROJECT_ID}.log}"

exec > >(tee -a "$LOG_FILE") 2>&1

echo "=== [$(date '+%F %T')] start rerun from episodes for project ${PROJECT_ID} ==="

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
  local body="${2:-{}}"
  curl -sS -X POST "${BASE_URL}${path}" \
    -H "$(auth_header)" \
    -H "Content-Type: application/json" \
    -d "${body}" \
    -w "\nHTTP %{http_code}\n"
}

echo "[$(date '+%F %T')] reset project status for re-split"
docker exec autovideo-postgres psql -U postgres -d project_db -v ON_ERROR_STOP=1 -c \
  "UPDATE projects SET status='script_ready' WHERE id=${PROJECT_ID};"

echo "[$(date '+%F %T')] phase 0: rebuild episodes from script (clears old storyboards)"
api_post "/api/v1/projects/${PROJECT_ID}/episodes/generate" '{"rebuild":true}'

wait_episode_split_done() {
  local tries=0
  while (( tries < 360 )); do
    local row
    row=$(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -F '|' -c \
      "SELECT status, COALESCE(progress::text,'') FROM projects WHERE id=${PROJECT_ID};" | head -1)
    local status="${row%%|*}"
    local ep_count
    ep_count=$(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -c \
      "SELECT COUNT(*) FROM episodes WHERE project_id=${PROJECT_ID};" | tr -d '[:space:]')
    echo "[$(date '+%F %T')] episode_split status=${status} episodes=${ep_count}"
    if [[ "$status" == "script_ready" || "$status" == "storyboard_generating" ]] && [[ "${ep_count:-0}" -gt 0 ]]; then
      return 0
    fi
    if [[ "$status" == "failed" ]]; then
      echo "[$(date '+%F %T')] project status failed during episode split"
      return 1
    fi
    sleep 10
    tries=$((tries + 1))
  done
  echo "[$(date '+%F %T')] episode split timed out"
  return 1
}

wait_episode_split_done

echo "[$(date '+%F %T')] reset episode optimize/review status"
docker exec autovideo-postgres psql -U postgres -d project_db -v ON_ERROR_STOP=1 -c \
  "UPDATE episodes SET optimize_status='pending', review_status='pending', status='pending' WHERE project_id=${PROJECT_ID};"

EPISODE_IDS=()
while IFS= read -r line; do
  [[ -n "$line" ]] && EPISODE_IDS+=("$line")
done < <(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -c \
  "SELECT id FROM episodes WHERE project_id=${PROJECT_ID} ORDER BY episode_number;")

echo "[$(date '+%F %T')] found ${#EPISODE_IDS[@]} episodes"

wait_episode_done() {
  local eid="$1"
  local tries=0
  while (( tries < 240 )); do
    local row
    row=$(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -F '|' -c \
      "SELECT optimize_status, review_status FROM episodes WHERE id=${eid};" | tr -d '[:space:]')
    local opt="${row%%|*}"
    local rev="${row##*|}"
    if [[ "$opt" == "done" && ( "$rev" == "done" || "$rev" == "failed" ) ]]; then
      echo "[$(date '+%F %T')] episode ${eid} finished optimize=${opt} review=${rev}"
      return 0
    fi
    if [[ "$opt" == "failed" ]]; then
      echo "[$(date '+%F %T')] episode ${eid} optimize failed"
      return 1
    fi
    sleep 10
    tries=$((tries + 1))
  done
  echo "[$(date '+%F %T')] episode ${eid} timed out"
  return 1
}

echo "[$(date '+%F %T')] phase 1: auto-optimize-review (${PARALLEL} parallel)"
idx=0
while (( idx < ${#EPISODE_IDS[@]} )); do
  batch=()
  for ((j=0; j<PARALLEL && idx+j<${#EPISODE_IDS[@]}; j++)); do
    batch+=("${EPISODE_IDS[$((idx+j))]}")
  done

  for eid in "${batch[@]}"; do
    echo "[$(date '+%F %T')] trigger auto-optimize-review episode_id=${eid}"
    api_post "/api/v1/projects/${PROJECT_ID}/episodes/${eid}/auto-optimize-review" || true
  done

  for eid in "${batch[@]}"; do
    wait_episode_done "$eid" || true
  done

  idx=$((idx + PARALLEL))
done

echo "[$(date '+%F %T')] optimize summary"
docker exec autovideo-postgres psql -U postgres -d project_db -c \
  "SELECT optimize_status, review_status, COUNT(*) FROM episodes WHERE project_id=${PROJECT_ID} GROUP BY 1,2 ORDER BY 1,2;"
docker exec autovideo-postgres psql -U postgres -d project_db -c \
  "SELECT COUNT(*) AS with_subtitle_tag FROM episodes WHERE project_id=${PROJECT_ID} AND optimized_text LIKE '%[字幕:%';"

echo "[$(date '+%F %T')] phase 2: extract storyboards (full project)"
api_post "/api/v1/projects/${PROJECT_ID}/episodes/extract-storyboards"

echo "[$(date '+%F %T')] waiting for storyboard extraction"
tries=0
while (( tries < 480 )); do
  running=$(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -c \
    "SELECT COUNT(*) FROM episodes WHERE project_id=${PROJECT_ID} AND status IN ('scene_splitting','processing');" | tr -d '[:space:]')
  sb_count=$(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -c \
    "SELECT COUNT(*) FROM storyboards WHERE project_id=${PROJECT_ID};" | tr -d '[:space:]')
  dlg_nonempty=$(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -c \
    "SELECT COUNT(*) FROM storyboards WHERE project_id=${PROJECT_ID} AND COALESCE(dialogue,'') <> '';" | tr -d '[:space:]')
  echo "[$(date '+%F %T')] storyboards=${sb_count} with_dialogue=${dlg_nonempty} running_episodes=${running}"
  if [[ "$running" == "0" && "$sb_count" != "0" ]]; then
    break
  fi
  sleep 15
  tries=$((tries + 1))
done

echo "[$(date '+%F %T')] storyboard dialogue stats (first 5 episodes)"
docker exec autovideo-postgres psql -U postgres -d project_db -c \
  "SELECT e.episode_number,
          COUNT(*) AS scenes,
          SUM(LENGTH(COALESCE(sb.dialogue,''))) AS dlg_chars,
          COUNT(*) FILTER (WHERE COALESCE(sb.dialogue,'')='') AS empty_dlg
   FROM storyboards sb
   JOIN episodes e ON e.id = sb.episode_id
   WHERE e.project_id=${PROJECT_ID}
   GROUP BY e.episode_number
   ORDER BY e.episode_number
   LIMIT 5;"

echo "=== [$(date '+%F %T')] rerun finished; log=${LOG_FILE} ==="
