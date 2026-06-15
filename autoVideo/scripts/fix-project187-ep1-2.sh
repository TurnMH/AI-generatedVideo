#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID=187
EP1_ID=7163
EP2_ID=7164
BASE_URL="${BASE_URL:-http://localhost:8000}"
JWT_SECRET="${JWT_SECRET:-autovideo-access-secret-change-in-prod}"
USER_ID="${USER_ID:-6}"
LOG_FILE="${LOG_FILE:-/tmp/fix-project-${PROJECT_ID}-ep1-2.log}"

exec > >(tee -a "$LOG_FILE") 2>&1

echo "=== [$(date '+%F %T')] fix + verify project ${PROJECT_ID} episodes 1-2 ==="

TOKEN=$(python3 - <<PY
import jwt, time, os
print(jwt.encode({
    "user_id": int(os.environ.get("USER_ID", "6")),
    "role": "service",
    "exp": int(time.time()) + 8 * 3600,
}, os.environ.get("JWT_SECRET", "autovideo-access-secret-change-in-prod"), algorithm="HS256"))
PY
)

auth() { echo "Authorization: Bearer ${TOKEN}"; }

echo "[$(date '+%F %T')] step 1: fix storyboard dialogues"
docker exec -i autovideo-postgres psql -U postgres -d project_db -v ON_ERROR_STOP=1 <<'SQL'
UPDATE storyboards SET dialogue = '刘师傅神情沉默，缓缓解下围裙，叠好放在灶台上，离开时只带走一只黑色塑料桶。' WHERE id = 71908;
UPDATE storyboards SET dialogue = '三个月后，德聚楼因口味垮塌，订单大量退款，老板竟然跪在我包子铺门口，苦苦哀求:“刘师傅，求求你救救我！”' WHERE id = 71909;
UPDATE storyboards SET dialogue = '有人笑着说：“就带这破桶？”我没回头' WHERE id = 71917;
UPDATE storyboards SET dialogue = '我把面团盖上湿布，搬出两把塑料凳子，放到他们面前，说：“坐。”' WHERE id = 71920;
UPDATE storyboards SET dialogue = '就是他，我走那天，他顺手把我用了二十年的老铁勺扔进垃圾桶，说：“换新的，老古董碍事。”' WHERE id = 71922;
UPDATE storyboards SET dialogue = '刘师傅拉开躺椅，点上旱烟，神情沉稳，等对方开口。' WHERE id = 71923;
UPDATE storyboards SET dialogue = '说吧。' WHERE id = 71924;
UPDATE storyboards SET dialogue = '王总，三个月前你说机器分秒不差，我老了手抖，不如机器好用。' WHERE id = 71926;
UPDATE storyboards SET dialogue = '那这三个月，机器呢？' WHERE id = 71927;
SQL

echo "[$(date '+%F %T')] step 2: dedupe dubbing tasks"
docker exec -i autovideo-postgres psql -U postgres -d video_db -v ON_ERROR_STOP=1 <<'SQL'
DELETE FROM dubbing_tasks a
USING dubbing_tasks b
WHERE a.project_id = 187
  AND b.project_id = 187
  AND a.storyboard_id IS NOT NULL
  AND a.storyboard_id = b.storyboard_id
  AND a.id > b.id;
SQL

echo "[$(date '+%F %T')] step 3: trigger ep2 storyboard images"
curl -sS -X POST "${BASE_URL}/api/v1/projects/${PROJECT_ID}/storyboards/generate-all" \
  -H "$(auth)" \
  -H "Content-Type: application/json" \
  -d "{\"episode_id\": ${EP2_ID}}" \
  -w "\nHTTP %{http_code}\n"

echo "[$(date '+%F %T')] step 4: regenerate ep1 video with story-aligned scene descriptions"
python3 - <<'PY' | curl -sS -X POST "${BASE_URL}/api/v1/projects/${PROJECT_ID}/videos/generate" \
  -H "$(auth)" \
  -H "Content-Type: application/json" \
  -d @- \
  -w "\nHTTP %{http_code}\n"
import json, subprocess

def q(sql):
    out = subprocess.check_output([
        "docker", "exec", "autovideo-postgres", "psql", "-U", "postgres", "-d", "project_db",
        "-t", "-A", "-F", "\t", "-c", sql,
    ], text=True)
    return [line.split("\t") for line in out.strip().splitlines() if line.strip()]

rows_json = subprocess.check_output([
        "docker", "exec", "autovideo-postgres", "psql", "-U", "postgres", "-d", "project_db",
        "-t", "-A", "-c", """
SELECT COALESCE(json_agg(row_to_json(t)), '[]'::json)
FROM (
  SELECT s.image_url, s.scene_description, s.dialogue, s.prompt_used,
         COALESCE(s.duration,0) AS duration,
         COALESCE(s.camera_movement,'') AS camera_movement,
         COALESCE(s.mood,'') AS mood,
         COALESCE(s.characters, ARRAY[]::text[]) AS characters,
         COALESCE(s.asset_ids, ARRAY[]::bigint[]) AS asset_ids
  FROM storyboards s
  JOIN episodes e ON e.id = s.episode_id
  WHERE s.project_id = 187 AND e.episode_number = 1 AND s.image_url <> ''
  ORDER BY s.sequence_number
) t;
""",
    ], text=True).strip()
rows = json.loads(rows_json or "[]")


def build_desc(scene, dialogue, prompt):
    scene = (scene or "").strip()
    dialogue = (dialogue or "").strip()
    prompt = (prompt or "").strip()
    desc = scene or prompt
    if not desc and dialogue:
        return f"剧情节拍：{dialogue}"
    if dialogue and dialogue not in desc:
        desc = f"{desc}。本镜旁白/对白：{dialogue}"
    return desc


payload = {
    "episode_id": 7163,
    "image_urls": [],
    "scene_descriptions": [],
    "dialogues": [],
    "durations": [],
    "camera_movements": [],
    "moods": [],
    "scene_characters": [],
    "scene_asset_ids": [],
    "model_name": "wan",
    "style_preset": "anime-3d",
    "motion_mode": "gentle",
    "video_mode": "api_generation",
    "clip_duration_sec": 4,
    "render_config": {
        "production_mode": "commentary_comic",
        "duration": "4",
        "aspect_ratio": "9:16",
    },
}

for row in rows:
    payload["image_urls"].append(row["image_url"])
    payload["scene_descriptions"].append(build_desc(row.get("scene_description"), row.get("dialogue"), row.get("prompt_used")))
    payload["dialogues"].append((row.get("dialogue") or "").strip())
    payload["durations"].append(float(row.get("duration") or 4))
    payload["camera_movements"].append(row.get("camera_movement") or "")
    payload["moods"].append(row.get("mood") or "")
    payload["scene_characters"].append(row.get("characters") or [])
    payload["scene_asset_ids"].append([int(x) for x in (row.get("asset_ids") or [])])

payload["scene_description"] = " ".join([d for d in payload["scene_descriptions"] if d])
print(json.dumps(payload, ensure_ascii=False))
PY

echo "[$(date '+%F %T')] step 5: regenerate ep1 per-storyboard dubbing"
for sid in 71906 71907 71908 71909 71910 71911 71912 71913 71914; do
  echo "  dubbing storyboard ${sid}"
  curl -sS -X POST "${BASE_URL}/api/v1/projects/${PROJECT_ID}/storyboards/${sid}/dubbing" \
    -H "$(auth)" \
    -H "Content-Type: application/json" \
    -d '{}' \
    -w " HTTP %{http_code}\n" || true
  sleep 0.3
done

echo "[$(date '+%F %T')] done — log: ${LOG_FILE}"
