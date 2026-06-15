#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID=188
EP1_ID=7230
BASE_URL="${BASE_URL:-http://localhost:8000}"
JWT_SECRET="${JWT_SECRET:-autovideo-access-secret-change-in-prod}"
USER_ID="${USER_ID:-6}"
LOG_FILE="${LOG_FILE:-/tmp/check-project-${PROJECT_ID}-ep1.log}"

exec > >(tee -a "$LOG_FILE") 2>&1

echo "=== [$(date '+%F %T')] audit + verify project ${PROJECT_ID} episode 1 ==="

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

echo "[$(date '+%F %T')] step 1: audit project + episodes"
docker exec autovideo-postgres psql -U postgres -d project_db -c "
SELECT id, title, mode, target_episodes,
       storyboard_config->>'production_mode' AS production_mode,
       storyboard_config->>'video_model' AS video_model,
       progress->>'message' AS progress_message
FROM projects WHERE id = ${PROJECT_ID};"

docker exec autovideo-postgres psql -U postgres -d project_db -c "
SELECT episode_number, id, optimize_status, review_status, word_count,
       LENGTH(COALESCE(original_excerpt,'')) AS orig_len,
       (script_excerpt = optimized_text) AS opt_matches_excerpt
FROM episodes WHERE project_id = ${PROJECT_ID} ORDER BY episode_number LIMIT 5;"

echo "[$(date '+%F %T')] step 2: repair ep1 script from source chapter 01 (strip broken LLM tags)"
python3 - <<'PY' | docker exec -i autovideo-postgres psql -U postgres -d project_db -v ON_ERROR_STOP=1
import re, subprocess, json

def q(sql, *args):
    if args:
        sql = sql % args
    out = subprocess.check_output([
        "docker", "exec", "autovideo-postgres", "psql", "-U", "postgres", "-d", "project_db",
        "-t", "-A", "-c", sql,
    ], text=True)
    return out.strip()

script = q("SELECT script_text FROM projects WHERE id=188")
script = script.replace("\r\n", "\n")

m01 = re.search(r"(?m)^01\s*$", script)
m02 = re.search(r"(?m)^02\s*$", script)
if not m01 or not m02:
    raise SystemExit("chapter markers 01/02 not found")
ch01 = script[m01.end():m02.start()].strip()

prologue = ""
pm = re.search(r"【导语】\s*(.*?)(?=\n01\s*$)", script, re.S)
if pm:
    prologue = re.sub(r"\s+", " ", pm.group(1).strip())

paragraphs = [p.strip() for p in re.split(r"\n\s*\n", ch01) if p.strip()]
if prologue:
    paragraphs.insert(0, prologue)

blocks = []
for para in paragraphs:
    speak = re.sub(r"\s+", " ", para).strip()
    action = ""
    if "揉面" in speak and "进门" in speak:
        action = "北街包子铺内，刘师傅低头揉面，王大发站在门口。"
    elif "解下围裙" in speak or "塑料桶" in speak and "撵走" in speak:
        action = "回忆画面，德聚楼后厨，王大发拍钱在灶台上，刘师傅解下围裙叠好，只带走空塑料桶。"
    elif "就带这破桶" in speak:
        action = "包子铺门口，有人嘲笑，刘师傅没回头。"
    elif "搬出两把塑料凳子" in speak or '"坐。"' in speak or "坐。" in speak:
        action = "包子铺内，刘师傅盖上面团湿布，搬出塑料凳子。"
    elif "陈大鹏" in speak and "厨师长" in speak:
        action = "包子铺内，王大发身后跟着陈大鹏，刘师傅点旱烟。"
    elif "说吧" in speak or "专程道歉" in speak:
        action = "包子铺内，三人相对而坐，王大发赔笑开口。"
    elif "机器分秒不差" in speak or "那这三个月" in speak:
        action = "刘师傅坐直身子，语气冷静。"
    elif "经验主义" in speak or "你说个数" in speak:
        action = "陈大鹏插话，刘师傅低头点烟。"
    elif "塑料桶" in speak and "认得" in speak:
        action = "刘师傅拿起门口的空塑料桶，放到两人面前。"
    else:
        action = "北街包子铺，清晨，人物动作连贯。"
    blocks.append(f"[字幕:{speak}][场景:{action}]")

repaired = "\n\n".join(blocks)
repaired_sql = repaired.replace("'", "''")

print(f"SELECT 'repaired_chars', {len(repaired)};")
print(f"UPDATE episodes SET script_excerpt = '{repaired_sql}', optimized_text = '{repaired_sql}', word_count = {len(repaired)}, optimize_status = 'done', review_status = 'done', updated_at = NOW() WHERE id = 7230;")

checks = ["刘师傅。", "就带这破桶？", "坐。", "换新的，老古董碍事。", "王总，你认得这个吗？"]
for c in checks:
    print(f"SELECT 'check_{c[:8]}', POSITION('{c.replace(chr(39), chr(39)+chr(39))}' IN script_excerpt) > 0 FROM episodes WHERE id=7230;")
PY

echo "[$(date '+%F %T')] step 3: trigger ep1 auto-pipeline (资源 + 分镜)"
HTTP=$(curl -sS -o /tmp/p188-auto-pipeline.json -w "%{http_code}" -X POST \
  "${BASE_URL}/api/v1/projects/${PROJECT_ID}/episodes/${EP1_ID}/auto-pipeline" \
  -H "$(auth)" \
  -H "Content-Type: application/json")
echo "auto-pipeline HTTP ${HTTP}"
cat /tmp/p188-auto-pipeline.json || true
echo

echo "[$(date '+%F %T')] step 4: poll storyboards (max 3 min)"
for i in $(seq 1 18); do
  sleep 10
  read -r SB IMGS <<< "$(docker exec autovideo-postgres psql -U postgres -d project_db -t -A -c "
    SELECT COUNT(*), COUNT(*) FILTER (WHERE image_url IS NOT NULL AND image_url <> '')
    FROM storyboards WHERE project_id=${PROJECT_ID} AND episode_id=${EP1_ID};" | tr '|' ' ')"
  echo "[$(date '+%F %T')] poll ${i}: storyboards=${SB} images=${IMGS}"
  if [[ "${SB:-0}" -gt 0 && "${IMGS:-0}" == "${SB:-0}" ]]; then
    break
  fi
done

echo "[$(date '+%F %T')] step 5: sample storyboard dialogue/scene lengths"
docker exec autovideo-postgres psql -U postgres -d project_db -c "
SELECT sequence_number,
       LENGTH(COALESCE(dialogue,'')) AS dlg_len,
       LENGTH(COALESCE(scene_description,'')) AS scene_len,
       LEFT(COALESCE(dialogue,''), 80) AS dialogue_preview,
       LEFT(COALESCE(scene_description,''), 80) AS scene_preview
FROM storyboards
WHERE project_id=${PROJECT_ID} AND episode_id=${EP1_ID}
ORDER BY sequence_number
LIMIT 8;"

echo "=== done. log: ${LOG_FILE} ==="
