#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BACKUP_PATH="${1:?usage: dev-restore.sh <backup-directory> --yes}"
CONFIRM="${2:-}"
MONGO_CONTAINER="${MONGO_CONTAINER:-mongo-dev}"
SEAWEED_VOLUME="${SEAWEED_VOLUME:-snappsy-server_seaweedfs_data_dev}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dev.yml}"

if [[ "$CONFIRM" != "--yes" ]]; then
  echo "This replaces local dev MongoDB and SeaweedFS data."
  echo "Re-run with --yes to continue."
  exit 1
fi

[[ -f "$BACKUP_PATH/mongo.archive.gz" ]] || { echo "missing mongo archive" >&2; exit 1; }
[[ -f "$BACKUP_PATH/seaweedfs.tar.gz" ]] || { echo "missing seaweedfs archive" >&2; exit 1; }

echo "stopping local dev data services"
docker compose -f "$COMPOSE_FILE" stop boomerang-worker seaweedfs mongo

echo "restoring SeaweedFS volume $SEAWEED_VOLUME"
docker run --rm \
  -v "$SEAWEED_VOLUME:/data" \
  -v "$(cd "$BACKUP_PATH" && pwd):/backup:ro" \
  alpine:3.20 \
  sh -c 'rm -rf /data/* && tar xzf /backup/seaweedfs.tar.gz -C /data'

echo "starting MongoDB"
docker compose -f "$COMPOSE_FILE" up -d mongo

echo "waiting for MongoDB"
for _ in $(seq 1 30); do
  if docker exec "$MONGO_CONTAINER" sh -c 'mongosh --quiet --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --eval "db.runCommand({ ping: 1 })"' >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "restoring MongoDB"
docker exec -i "$MONGO_CONTAINER" sh -c \
  'mongorestore --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --drop --gzip --archive' \
  < "$BACKUP_PATH/mongo.archive.gz"

echo "starting SeaweedFS and worker"
docker compose -f "$COMPOSE_FILE" up -d seaweedfs boomerang-worker

echo "restore complete. run ./scripts/smoke.sh and check a media URL."
