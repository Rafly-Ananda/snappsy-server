#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MONGO_CONTAINER="${MONGO_CONTAINER:-mongo-dev}"
SEAWEED_VOLUME="${SEAWEED_VOLUME:-snappsy-server_seaweedfs_data_dev}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$BACKUP_DIR/dev-$STAMP"

mkdir -p "$OUT"

echo "backing up MongoDB from $MONGO_CONTAINER"
docker exec "$MONGO_CONTAINER" sh -c \
  'mongodump --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --db snappsy --archive --gzip' \
  > "$OUT/mongo.archive.gz"

echo "backing up SeaweedFS volume $SEAWEED_VOLUME"
docker run --rm \
  -v "$SEAWEED_VOLUME:/data:ro" \
  -v "$(cd "$OUT" && pwd):/backup" \
  alpine:3.20 \
  tar czf /backup/seaweedfs.tar.gz -C /data .

cat > "$OUT/manifest.txt" <<EOF
created_at=$STAMP
mongo_container=$MONGO_CONTAINER
seaweed_volume=$SEAWEED_VOLUME
note=local development drill only
EOF

echo "backup written to $OUT"
echo "copy it off this machine before treating it as a real backup."
