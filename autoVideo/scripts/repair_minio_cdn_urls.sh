#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PROJECT_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)

CONFIG_FILE=${CONFIG_FILE:-"$PROJECT_ROOT/config.docker.local.yaml"}
DB_CONTAINER=${DB_CONTAINER:-autovideo-postgres}
DB_USER=${DB_USER:-postgres}
MINIO_CONTAINER=${MINIO_CONTAINER:-autovideo-minio}
BAD_BASE=${BAD_BASE:-http://minio:9000}
APPLY=0

usage() {
  cat <<'EOF'
Usage: repair_minio_cdn_urls.sh [--apply] [--config PATH] [--bad-base URL]

Default mode is dry-run: print impacted row counts and distinct bad URLs.

Options:
  --apply         Copy bad objects from local MinIO to configured target storage,
                  then rewrite persisted URLs in Postgres.
  --config PATH   YAML config file to read storage target settings from.
  --bad-base URL  Bad URL base to replace. Default: http://minio:9000
  -h, --help      Show this message.
EOF
}

log() {
  printf '[repair-minio-cdn] %s\n' "$*"
}

die() {
  printf '[repair-minio-cdn] ERROR: %s\n' "$*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --apply)
      APPLY=1
      shift
      ;;
    --config)
      [[ $# -ge 2 ]] || die "--config requires a path"
      CONFIG_FILE=$2
      shift 2
      ;;
    --bad-base)
      [[ $# -ge 2 ]] || die "--bad-base requires a URL"
      BAD_BASE=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[[ -f "$CONFIG_FILE" ]] || die "config file not found: $CONFIG_FILE"
command -v docker >/dev/null 2>&1 || die "docker is required"
command -v awk >/dev/null 2>&1 || die "awk is required"
command -v sed >/dev/null 2>&1 || die "sed is required"
command -v sort >/dev/null 2>&1 || die "sort is required"

BAD_BASE=${BAD_BASE%/}

yaml_get() {
  local path=$1
  awk -v target="$path" '
    function ltrim(s) { sub(/^[[:space:]]+/, "", s); return s }
    function rtrim(s) { sub(/[[:space:]]+$/, "", s); return s }
    function trim(s)  { return rtrim(ltrim(s)) }
    BEGIN { n = split(target, parts, ".") }
    {
      line = $0
      sub(/\r$/, "", line)
      if (match(line, /^([ ]*)([^:#][^:]*):[ ]*(.*)$/, m)) {
        indent = length(m[1]) / 2
        key = trim(m[2])
        value = m[3]
        stack[indent] = key
        for (i = indent + 1; i < 32; i++) {
          delete stack[i]
        }
        ok = 1
        for (i = 1; i <= n; i++) {
          if (!(i - 1 in stack) || stack[i - 1] != parts[i]) {
            ok = 0
            break
          }
        }
        if (ok && indent == n - 1) {
          value = trim(value)
          sub(/[[:space:]]+#.*$/, "", value)
          print value
          exit
        }
      }
    }
  ' "$CONFIG_FILE" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//"
}

psql_query() {
  local db=$1
  local sql=$2
  docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$db" -v ON_ERROR_STOP=1 -tAc "$sql"
}

CDN_BASE_URL=$(yaml_get storage-service.storage.cdn_base_url)
[[ -n "$CDN_BASE_URL" ]] || die "storage-service.storage.cdn_base_url is empty"
CDN_BASE_URL=${CDN_BASE_URL%/}

MINIO_ENDPOINT=$(yaml_get storage-service.storage.minio.endpoint)
MINIO_ACCESS_KEY=$(yaml_get storage-service.storage.minio.access_key)
MINIO_SECRET_KEY=$(yaml_get storage-service.storage.minio.secret_key)
MINIO_USE_SSL=$(yaml_get storage-service.storage.minio.use_ssl)
MINIO_PATH_STYLE=$(yaml_get storage-service.storage.minio.path_style)
CDN_INCLUDE_BUCKET=$(yaml_get storage-service.storage.cdn_include_bucket)

BUCKET_IMAGES=$(yaml_get storage-service.storage.buckets.images)
BUCKET_VIDEOS=$(yaml_get storage-service.storage.buckets.videos)
BUCKET_SCRIPTS=$(yaml_get storage-service.storage.buckets.scripts)
BUCKET_CHARACTERS=$(yaml_get storage-service.storage.buckets.characters)
BUCKET_UPLOADS=$(yaml_get storage-service.storage.buckets.uploads)
BUCKET_EXPORTS=$(yaml_get storage-service.storage.buckets.exports)
BUCKET_DUBBING=$(yaml_get storage-service.storage.buckets.dubbing)
BUCKET_AUDIOS=$(yaml_get storage-service.storage.buckets.audios)

[[ -n "$MINIO_ENDPOINT" ]] || die "storage-service.storage.minio.endpoint is empty"
[[ -n "$MINIO_ACCESS_KEY" ]] || die "storage-service.storage.minio.access_key is empty"
[[ -n "$MINIO_SECRET_KEY" ]] || die "storage-service.storage.minio.secret_key is empty"

bucket_for() {
  case "$1" in
    images) echo "$BUCKET_IMAGES" ;;
    videos) echo "$BUCKET_VIDEOS" ;;
    scripts) echo "$BUCKET_SCRIPTS" ;;
    characters) echo "$BUCKET_CHARACTERS" ;;
    uploads) echo "$BUCKET_UPLOADS" ;;
    exports) echo "$BUCKET_EXPORTS" ;;
    dubbing) echo "$BUCKET_DUBBING" ;;
    audios) echo "$BUCKET_AUDIOS" ;;
    *) echo "$1" ;;
  esac
}

good_prefix_for_bucket() {
  local source_bucket=$1
  local target_bucket
  target_bucket=$(bucket_for "$source_bucket")
  if [[ "$CDN_INCLUDE_BUCKET" == "true" ]]; then
    printf '%s/%s/' "$CDN_BASE_URL" "$target_bucket"
  else
    printf '%s/' "$CDN_BASE_URL"
  fi
}

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

counts_file=$tmp_dir/counts.tsv
urls_file=$tmp_dir/urls.txt
objects_file=$tmp_dir/objects.tsv
source_buckets_file=$tmp_dir/source_buckets.txt

append_urls() {
  local db=$1
  local sql=$2
  psql_query "$db" "$sql" | sed '/^[[:space:]]*$/d' >> "$urls_file"
}

record_count() {
  local label=$1
  local db=$2
  local sql=$3
  local count
  count=$(psql_query "$db" "$sql")
  printf '%s\t%s\n' "$label" "${count:-0}" >> "$counts_file"
}

: > "$counts_file"
: > "$urls_file"

record_count assets.image_url character_db "SELECT COUNT(*) FROM assets WHERE image_url LIKE '${BAD_BASE}/%';"
record_count assets.composite_image_url character_db "SELECT COUNT(*) FROM assets WHERE composite_image_url LIKE '${BAD_BASE}/%';"
record_count assets.panel_images character_db "SELECT COUNT(*) FROM assets a WHERE EXISTS (SELECT 1 FROM unnest(a.panel_images) AS elem WHERE elem LIKE '${BAD_BASE}/%');"
record_count image_tasks.result_url image_db "SELECT COUNT(*) FROM image_tasks WHERE result_url LIKE '${BAD_BASE}/%';"
record_count image_tasks.thumbnail_url image_db "SELECT COUNT(*) FROM image_tasks WHERE thumbnail_url LIKE '${BAD_BASE}/%';"
record_count storyboards.image_url project_db "SELECT COUNT(*) FROM storyboards WHERE image_url LIKE '${BAD_BASE}/%';"
record_count storyboards.end_frame_image_url project_db "SELECT COUNT(*) FROM storyboards WHERE end_frame_image_url LIKE '${BAD_BASE}/%';"
record_count storyboard_versions.image_url project_db "SELECT COUNT(*) FROM storyboard_versions WHERE image_url LIKE '${BAD_BASE}/%';"
record_count files.cdn_url storage_db "SELECT COUNT(*) FROM files WHERE cdn_url LIKE '${BAD_BASE}/%';"

append_urls character_db "SELECT image_url FROM assets WHERE image_url LIKE '${BAD_BASE}/%';"
append_urls character_db "SELECT composite_image_url FROM assets WHERE composite_image_url LIKE '${BAD_BASE}/%';"
append_urls character_db "SELECT elem FROM assets a, unnest(a.panel_images) AS elem WHERE elem LIKE '${BAD_BASE}/%';"
append_urls image_db "SELECT result_url FROM image_tasks WHERE result_url LIKE '${BAD_BASE}/%';"
append_urls image_db "SELECT thumbnail_url FROM image_tasks WHERE thumbnail_url LIKE '${BAD_BASE}/%';"
append_urls project_db "SELECT image_url FROM storyboards WHERE image_url LIKE '${BAD_BASE}/%';"
append_urls project_db "SELECT end_frame_image_url FROM storyboards WHERE end_frame_image_url LIKE '${BAD_BASE}/%';"
append_urls project_db "SELECT image_url FROM storyboard_versions WHERE image_url LIKE '${BAD_BASE}/%';"
append_urls storage_db "SELECT cdn_url FROM files WHERE cdn_url LIKE '${BAD_BASE}/%';"

LC_ALL=C sort -u "$urls_file" -o "$urls_file"

log "bad URL row counts"
cat "$counts_file"

if [[ ! -s "$urls_file" ]]; then
  log "no persisted URLs matched ${BAD_BASE}/"
  exit 0
fi

log "sample bad URLs"
head -n 10 "$urls_file"

: > "$objects_file"
while IFS= read -r url; do
  [[ -n "$url" ]] || continue
  path=${url#"$BAD_BASE/"}
  source_bucket=${path%%/*}
  object_key=${path#*/}
  if [[ -z "$source_bucket" || -z "$object_key" || "$source_bucket" == "$path" ]]; then
    die "unable to parse bad URL: $url"
  fi
  printf '%s\t%s\t%s\n' "$source_bucket" "$object_key" "$(bucket_for "$source_bucket")" >> "$objects_file"
done < "$urls_file"

LC_ALL=C sort -u "$objects_file" -o "$objects_file"
cut -f1 "$objects_file" | LC_ALL=C sort -u > "$source_buckets_file"

log "distinct broken objects: $(wc -l < "$objects_file" | tr -d ' ')"

if [[ $APPLY -ne 1 ]]; then
  log "dry-run complete; rerun with --apply to copy objects and rewrite URLs"
  exit 0
fi

DOCKER_NETWORK=$(docker inspect "$MINIO_CONTAINER" --format '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' | head -n 1 | tr -d '[:space:]')
[[ -n "$DOCKER_NETWORK" ]] || die "failed to detect docker network from $MINIO_CONTAINER"

if [[ "$MINIO_USE_SSL" == "true" ]]; then
  TARGET_SCHEME=https
else
  TARGET_SCHEME=http
fi

if [[ "$MINIO_PATH_STYLE" == "true" ]]; then
  MC_PATH_MODE=on
else
  MC_PATH_MODE=auto
fi

log "copying bad objects into configured target storage"
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --entrypoint /bin/sh \
  -v "$objects_file:/tmp/objects.tsv:ro" \
  -e TARGET_SCHEME="$TARGET_SCHEME" \
  -e TARGET_ENDPOINT="$MINIO_ENDPOINT" \
  -e TARGET_ACCESS_KEY="$MINIO_ACCESS_KEY" \
  -e TARGET_SECRET_KEY="$MINIO_SECRET_KEY" \
  -e MC_PATH_MODE="$MC_PATH_MODE" \
  minio/mc:latest \
  -ceu '
    mc alias set src http://minio:9000 minioadmin minioadmin --path on >/dev/null
    mc alias set dst "$TARGET_SCHEME://$TARGET_ENDPOINT" "$TARGET_ACCESS_KEY" "$TARGET_SECRET_KEY" --path "$MC_PATH_MODE" >/dev/null
    while IFS="$(printf "\t")" read -r source_bucket object_key target_bucket; do
      [ -n "$source_bucket" ] || continue
      mc stat "src/$source_bucket/$object_key" >/dev/null
      if ! mc stat "dst/$target_bucket/$object_key" >/dev/null 2>&1; then
        mc cp "src/$source_bucket/$object_key" "dst/$target_bucket/$object_key" >/dev/null
      fi
    done < /tmp/objects.tsv
  '

build_rewrite_sql() {
  local table_name=$1
  local column_name=$2
  local sql="BEGIN;\n"
  while IFS= read -r source_bucket; do
    [[ -n "$source_bucket" ]] || continue
    local bad_prefix=${BAD_BASE}/${source_bucket}/
    local good_prefix
    good_prefix=$(good_prefix_for_bucket "$source_bucket")
    sql+="UPDATE ${table_name} SET ${column_name} = REPLACE(${column_name}, '${bad_prefix}', '${good_prefix}') WHERE ${column_name} LIKE '${bad_prefix}%';\n"
  done < "$source_buckets_file"
  sql+="COMMIT;"
  printf '%b' "$sql"
}

build_panel_images_sql() {
  local sql="BEGIN;\n"
  while IFS= read -r source_bucket; do
    [[ -n "$source_bucket" ]] || continue
    local bad_prefix=${BAD_BASE}/${source_bucket}/
    local good_prefix
    good_prefix=$(good_prefix_for_bucket "$source_bucket")
    sql+="UPDATE assets SET panel_images = ARRAY(SELECT CASE WHEN elem LIKE '${bad_prefix}%' THEN REPLACE(elem, '${bad_prefix}', '${good_prefix}') ELSE elem END FROM unnest(panel_images) AS elem) WHERE EXISTS (SELECT 1 FROM unnest(panel_images) AS elem WHERE elem LIKE '${bad_prefix}%');\n"
  done < "$source_buckets_file"
  sql+="COMMIT;"
  printf '%b' "$sql"
}

log "rewriting persisted URLs"
psql_query character_db "$(build_rewrite_sql assets image_url)"
psql_query character_db "$(build_rewrite_sql assets composite_image_url)"
psql_query character_db "$(build_panel_images_sql)"
psql_query image_db "$(build_rewrite_sql image_tasks result_url)"
psql_query image_db "$(build_rewrite_sql image_tasks thumbnail_url)"
psql_query project_db "$(build_rewrite_sql storyboards image_url)"
psql_query project_db "$(build_rewrite_sql storyboards end_frame_image_url)"
psql_query project_db "$(build_rewrite_sql storyboard_versions image_url)"
psql_query storage_db "$(build_rewrite_sql files cdn_url)"

log "post-rewrite verification"
cat <<EOF
assets.image_url	$(psql_query character_db "SELECT COUNT(*) FROM assets WHERE image_url LIKE '${BAD_BASE}/%';")
assets.composite_image_url	$(psql_query character_db "SELECT COUNT(*) FROM assets WHERE composite_image_url LIKE '${BAD_BASE}/%';")
assets.panel_images	$(psql_query character_db "SELECT COUNT(*) FROM assets a WHERE EXISTS (SELECT 1 FROM unnest(a.panel_images) AS elem WHERE elem LIKE '${BAD_BASE}/%');")
image_tasks.result_url	$(psql_query image_db "SELECT COUNT(*) FROM image_tasks WHERE result_url LIKE '${BAD_BASE}/%';")
image_tasks.thumbnail_url	$(psql_query image_db "SELECT COUNT(*) FROM image_tasks WHERE thumbnail_url LIKE '${BAD_BASE}/%';")
storyboards.image_url	$(psql_query project_db "SELECT COUNT(*) FROM storyboards WHERE image_url LIKE '${BAD_BASE}/%';")
storyboards.end_frame_image_url	$(psql_query project_db "SELECT COUNT(*) FROM storyboards WHERE end_frame_image_url LIKE '${BAD_BASE}/%';")
storyboard_versions.image_url	$(psql_query project_db "SELECT COUNT(*) FROM storyboard_versions WHERE image_url LIKE '${BAD_BASE}/%';")
files.cdn_url	$(psql_query storage_db "SELECT COUNT(*) FROM files WHERE cdn_url LIKE '${BAD_BASE}/%';")
EOF