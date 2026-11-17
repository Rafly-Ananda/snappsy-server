#!/usr/bin/env bash
set -euo pipefail

MONGODB_URI="${MONGODB_URI:?set MONGODB_URI}"
S3_ENDPOINT="${S3_ENDPOINT:?set S3_ENDPOINT}"
S3_BUCKET="${S3_BUCKET:?set S3_BUCKET}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$BACKUP_DIR/$STAMP"

command -v mongodump >/dev/null || { echo "mongodump is required" >&2; exit 1; }
command -v aws >/dev/null || { echo "aws cli is required" >&2; exit 1; }

mkdir -p "$OUT/objects"
echo "dumping MongoDB"
mongodump --uri "$MONGODB_URI" --gzip --archive="$OUT/mongo.archive.gz"

echo "syncing SeaweedFS bucket"
aws --endpoint-url "$S3_ENDPOINT" s3 sync "s3://$S3_BUCKET" "$OUT/objects" --only-show-errors

cat > "$OUT/manifest.txt" <<EOF
created_at=$STAMP
mongodb=$MONGODB_URI
s3_endpoint=$S3_ENDPOINT
s3_bucket=$S3_BUCKET
EOF

echo "backup written to $OUT"
echo "keep the backup off-host; this directory is not a backup destination."
