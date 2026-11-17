#!/usr/bin/env bash
set -euo pipefail

MONGODB_URI="${MONGODB_URI:?set MONGODB_URI}"
S3_ENDPOINT="${S3_ENDPOINT:?set S3_ENDPOINT}"
S3_BUCKET="${S3_BUCKET:?set S3_BUCKET}"
BACKUP_PATH="${1:?usage: restore.sh <backup-directory>}"
DROP_FLAG=""

if [[ "${RESTORE_DROP:-0}" == "1" ]]; then
  DROP_FLAG="--drop"
  echo "RESTORE_DROP=1: existing MongoDB data will be replaced"
fi

[[ -f "$BACKUP_PATH/mongo.archive.gz" ]] || { echo "missing mongo archive" >&2; exit 1; }
[[ -d "$BACKUP_PATH/objects" ]] || { echo "missing object directory" >&2; exit 1; }

command -v mongorestore >/dev/null || { echo "mongorestore is required" >&2; exit 1; }
command -v aws >/dev/null || { echo "aws cli is required" >&2; exit 1; }

echo "restoring MongoDB"
mongorestore --uri "$MONGODB_URI" --gzip --archive="$BACKUP_PATH/mongo.archive.gz" $DROP_FLAG

echo "restoring SeaweedFS bucket"
aws --endpoint-url "$S3_ENDPOINT" s3 sync "$BACKUP_PATH/objects" "s3://$S3_BUCKET" --only-show-errors

echo "restore complete; verify health, media and display links before cutover"
