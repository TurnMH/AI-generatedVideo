#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID=187
BASE_URL="${BASE_URL:-http://localhost:8000}"
JWT_SECRET="${JWT_SECRET:-autovideo-access-secret-change-in-prod}"
USER_ID="${USER_ID:-6}"
MODEL_NAME="${MODEL_NAME:-vidu}"

TOKEN=$(python3 - <<PY
import jwt, time, os
print(jwt.encode({
    "user_id": int(os.environ.get("USER_ID", "6")),
    "role": "service",
    "exp": int(time.time()) + 8 * 3600,
}, os.environ.get("JWT_SECRET", "autovideo-access-secret-change-in-prod"), algorithm="HS256"))
PY
)

generate_episode_video() {
  local ep_num="$1"
  local episode_id="$2"
  echo "=== generating video for episode ${ep_num} (model=${MODEL_NAME}) ==="
  python3 - "$ep_num" "$episode_id" "$MODEL_NAME" <<'PY' | curl -sS -X POST "${BASE_URL}/api/v1/projects/${PROJECT_ID}/videos/generate" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d @-
import json, subprocess, sys

episode_num = int(sys.argv[1])
episode_id = int(sys.argv[2])
model_name = sys.argv[3]

def build_desc(scene, dialogue, prompt):
    scene = (scene or "").strip()
    dialogue = (dialogue or "").strip()
    prompt = (prompt or "").strip()
    desc = scene or prompt
    if not desc and dialogue:
        return f"剧情节拍：{dialogue}"
    if dialogue and dialogue not in desc:
        sep = "。" if desc and not desc.endswith(("。", ".", "!", "?", "！")) else ""
        desc = f"{desc}{sep}本镜旁白/对白：{dialogue}"
    return desc

rows_json = subprocess.check_output([
    "docker", "exec", "autovideo-postgres", "psql", "-U", "postgres", "-d", "project_db",
    "-t", "-A", "-c", f"""
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
  WHERE s.project_id = 187 AND e.episode_number = {episode_num}
    AND s.image_url <> '' AND s.status = 'completed'
  ORDER BY s.sequence_number
) t;
""",
], text=True).strip()
rows = json.loads(rows_json or "[]")
if not rows:
    raise SystemExit(f"no ready storyboards for episode {episode_num}")

payload = {
    "episode_id": episode_id,
    "image_urls": [],
    "scene_descriptions": [],
    "dialogues": [],
    "durations": [],
    "camera_movements": [],
    "moods": [],
    "scene_characters": [],
    "scene_asset_ids": [],
    "model_name": model_name,
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
  echo
}

generate_episode_video 1 7163
generate_episode_video 2 7164
