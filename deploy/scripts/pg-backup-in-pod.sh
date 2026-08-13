#!/bin/sh
# pg-backup-in-pod.sh — runs INSIDE the backup Job pod (postgres:16-alpine).
#
# Dumps the Beacon database and uploads a gzipped snapshot to object storage
# (MinIO for int/acc/test, S3 for prod). Both targets speak the S3 API, so one
# tool — the MinIO client `mc` — handles both; only the endpoint/creds differ.
#
# The Job injects these (see backup-postgres.sh):
#   PGHOST PGPORT PGUSER PGDATABASE PGPASSWORD   — the Beacon DB to dump
#   STORAGE_ENDPOINT STORAGE_ACCESS_KEY STORAGE_SECRET_KEY STORAGE_BUCKET — where to put it
#   TARGET_ENV        — names the object path (beacon/<env>/…)
#   RETENTION_DAYS    — snapshots older than this are pruned after a successful upload
set -eu

: "${PGHOST:?}"; : "${PGDATABASE:?}"; : "${PGUSER:?}"
: "${STORAGE_ENDPOINT:?}"; : "${STORAGE_ACCESS_KEY:?}"; : "${STORAGE_SECRET_KEY:?}"; : "${STORAGE_BUCKET:?}"
: "${TARGET_ENV:?}"; : "${RETENTION_DAYS:=14}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
DUMP="/tmp/beacon-${TARGET_ENV}-${TS}.sql"
OUT="/tmp/beacon-${TARGET_ENV}-${TS}.sql.gz"
PREFIX="beacon/${TARGET_ENV}"

# Dump to a file, THEN gzip — do not pipe. busybox/alpine sh has no reliable
# pipefail, so a failed pg_dump in `pg_dump | gzip` would still exit 0 and leave a
# truncated file that looks like a good backup. (Same reasoning as the platform's
# jobshout backup.)
echo "==> pg_dump ${PGDATABASE} @ ${PGHOST}:${PGPORT:-5432}"
pg_dump --no-owner --no-acl -h "$PGHOST" -p "${PGPORT:-5432}" -U "$PGUSER" -d "$PGDATABASE" -f "$DUMP"
gzip -c "$DUMP" > "${OUT}.partial"
mv "${OUT}.partial" "$OUT"          # only claim the final name once bytes are all there
rm -f "$DUMP"
SIZE="$(du -h "$OUT" | cut -f1)"
echo "==> dumped ${OUT} (${SIZE})"

# `mc` is a single static binary (Go); fetch it fresh so the image stays vanilla.
echo "==> fetching mc"
wget -q https://dl.min.io/client/mc/release/linux-amd64/mc -O /tmp/mc
chmod +x /tmp/mc

# STORAGE_ENDPOINT is a full URL (http://minio.svc:9000 or https://s3.<region>.amazonaws.com).
echo "==> uploading to ${STORAGE_BUCKET}/${PREFIX}/"
/tmp/mc alias set tgt "$STORAGE_ENDPOINT" "$STORAGE_ACCESS_KEY" "$STORAGE_SECRET_KEY" >/dev/null
/tmp/mc mb -p "tgt/${STORAGE_BUCKET}" >/dev/null 2>&1 || true   # no-op if it already exists
/tmp/mc cp "$OUT" "tgt/${STORAGE_BUCKET}/${PREFIX}/$(basename "$OUT")"

# Prune old snapshots only AFTER this one is safely uploaded, so a bad run can
# never delete history without first adding a good backup.
echo "==> pruning snapshots older than ${RETENTION_DAYS}d"
/tmp/mc rm --recursive --force --older-than "${RETENTION_DAYS}d" \
  "tgt/${STORAGE_BUCKET}/${PREFIX}/" >/dev/null 2>&1 || true

echo "OK: ${STORAGE_BUCKET}/${PREFIX}/$(basename "$OUT") (${SIZE})"
