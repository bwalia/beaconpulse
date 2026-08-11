#!/usr/bin/env bash
#
# backup-postgres.sh — back up one Beacon environment's Postgres to object storage.
#
# Runs on the self-hosted runner (needs kubectl). It does NOT dump/upload itself —
# it launches an in-cluster Job (which can reach the ClusterIP MinIO and the DB) and
# waits for it. The Job's logic lives in pg-backup-in-pod.sh.
#
# Storage routing (as requested): MinIO for int/acc/test, S3 for prod.
#   • MinIO envs reuse the in-cluster `workstation-minio` + `workstation-minio-creds`
#     that already exist in those namespaces — no new secrets.
#   • prod uploads to S3 using credentials passed in via the environment (the
#     workflow supplies them from GitHub secrets); we materialise a short-lived
#     Secret for the Job and delete it afterwards.
#
# Usage:
#   deploy/scripts/backup-postgres.sh <env>            # int|acc|test|prod (+ brand envs)
#   RETENTION_DAYS=14 deploy/scripts/backup-postgres.sh int
#
# Env for the prod (S3) path (from GitHub secrets):
#   BEACON_BACKUP_S3_ENDPOINT   e.g. https://s3.eu-west-2.amazonaws.com
#   BEACON_BACKUP_S3_BUCKET     e.g. beacon-prod-backups
#   BEACON_BACKUP_S3_ACCESS_KEY
#   BEACON_BACKUP_S3_SECRET_KEY
# Optional override for the MinIO envs:
#   BEACON_BACKUP_MINIO_BUCKET  (default: beacon-backups)
set -euo pipefail

ENV_ARG="${1:-}"
[ -n "$ENV_ARG" ] || { echo "usage: $0 <env> (int|acc|test|prod|<brand>-<tier>)" >&2; exit 2; }
NS="$ENV_ARG"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
MINIO_BUCKET="${BEACON_BACKUP_MINIO_BUCKET:-beacon-backups}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IN_POD="$REPO_ROOT/deploy/scripts/pg-backup-in-pod.sh"
STAMP="$(date -u +%Y%m%d%H%M%S)"
JOB="beacon-pg-backup-${STAMP}"
CM="beacon-pg-backup-script-${STAMP}"
S3_SECRET="beacon-pg-backup-s3-${STAMP}"

command -v kubectl >/dev/null || { echo "kubectl required" >&2; exit 1; }
[ -f "$IN_POD" ] || { echo "missing $IN_POD" >&2; exit 1; }

# Preflight: the env must actually run Beacon (its DB + secret must exist).
kubectl -n "$NS" get deploy beacon-postgres >/dev/null 2>&1 \
  || { echo "::error::no beacon-postgres deployment in namespace '$NS' — is Beacon deployed there?"; exit 1; }
kubectl -n "$NS" get secret beacon-secrets >/dev/null 2>&1 \
  || { echo "::error::no beacon-secrets in '$NS'"; exit 1; }

cleanup() {
  kubectl -n "$NS" delete job "$JOB" --ignore-not-found >/dev/null 2>&1 || true
  kubectl -n "$NS" delete configmap "$CM" --ignore-not-found >/dev/null 2>&1 || true
  kubectl -n "$NS" delete secret "$S3_SECRET" --ignore-not-found >/dev/null 2>&1 || true
}
trap cleanup EXIT

# The in-pod script travels as a ConfigMap so the image stays vanilla postgres:16-alpine.
kubectl -n "$NS" create configmap "$CM" --from-file=backup.sh="$IN_POD" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# ---- storage env sources: MinIO for lower tiers, S3 for prod ----------------
# A prod env is any whose tier is 'prod' (prod, sysops-prod, …).
case "$NS" in
  *prod)
    for v in BEACON_BACKUP_S3_ENDPOINT BEACON_BACKUP_S3_BUCKET BEACON_BACKUP_S3_ACCESS_KEY BEACON_BACKUP_S3_SECRET_KEY; do
      eval "val=\${$v:-}"; [ -n "$val" ] || { echo "::error::$v is required for the prod (S3) backup"; exit 1; }
    done
    # Short-lived Secret so the S3 creds never sit in the Job manifest / logs.
    kubectl -n "$NS" create secret generic "$S3_SECRET" \
      --from-literal=STORAGE_ENDPOINT="$BEACON_BACKUP_S3_ENDPOINT" \
      --from-literal=STORAGE_BUCKET="$BEACON_BACKUP_S3_BUCKET" \
      --from-literal=STORAGE_ACCESS_KEY="$BEACON_BACKUP_S3_ACCESS_KEY" \
      --from-literal=STORAGE_SECRET_KEY="$BEACON_BACKUP_S3_SECRET_KEY" \
      --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    # env: entries (12-space indent) — must align with the PG* vars above.
    STORAGE_ENVFROM="            - name: STORAGE_ENDPOINT
              valueFrom: { secretKeyRef: { name: ${S3_SECRET}, key: STORAGE_ENDPOINT } }
            - name: STORAGE_BUCKET
              valueFrom: { secretKeyRef: { name: ${S3_SECRET}, key: STORAGE_BUCKET } }
            - name: STORAGE_ACCESS_KEY
              valueFrom: { secretKeyRef: { name: ${S3_SECRET}, key: STORAGE_ACCESS_KEY } }
            - name: STORAGE_SECRET_KEY
              valueFrom: { secretKeyRef: { name: ${S3_SECRET}, key: STORAGE_SECRET_KEY } }"
    echo "env '$NS' -> S3 bucket '$BEACON_BACKUP_S3_BUCKET'"
    ;;
  *)
    kubectl -n "$NS" get secret workstation-minio-creds >/dev/null 2>&1 \
      || { echo "::error::no workstation-minio-creds in '$NS' — cannot back up to MinIO. Provide S3 creds or create the MinIO secret."; exit 1; }
    STORAGE_ENVFROM="            - name: STORAGE_ENDPOINT
              value: http://workstation-minio.${NS}.svc.cluster.local:9000
            - name: STORAGE_BUCKET
              value: ${MINIO_BUCKET}
            - name: STORAGE_ACCESS_KEY
              valueFrom: { secretKeyRef: { name: workstation-minio-creds, key: MINIO_ROOT_USER } }
            - name: STORAGE_SECRET_KEY
              valueFrom: { secretKeyRef: { name: workstation-minio-creds, key: MINIO_ROOT_PASSWORD } }"
    echo "env '$NS' -> MinIO bucket '$MINIO_BUCKET' (workstation-minio)"
    ;;
esac

# ---- render + apply the Job -------------------------------------------------
kubectl apply -f - >/dev/null <<YAML
apiVersion: batch/v1
kind: Job
metadata:
  name: ${JOB}
  namespace: ${NS}
  labels: { app.kubernetes.io/part-of: beacon, app.kubernetes.io/component: pg-backup }
spec:
  backoffLimit: 1
  ttlSecondsAfterFinished: 1800
  activeDeadlineSeconds: 1800
  template:
    metadata:
      labels: { app.kubernetes.io/part-of: beacon, app.kubernetes.io/component: pg-backup }
    spec:
      restartPolicy: Never
      containers:
        - name: backup
          image: postgres:16-alpine
          command: ["/bin/sh", "/scripts/backup.sh"]
          env:
            - name: TARGET_ENV
              value: "${NS}"
            - name: RETENTION_DAYS
              value: "${RETENTION_DAYS}"
            - name: PGHOST
              value: beacon-postgres
            - name: PGPORT
              value: "5432"
            - name: PGUSER
              value: beacon
            - name: PGDATABASE
              value: beacon
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef: { name: beacon-secrets, key: POSTGRES_PASSWORD }
${STORAGE_ENVFROM}
          volumeMounts:
            - name: script
              mountPath: /scripts
          resources:
            requests: { cpu: 50m, memory: 128Mi }
            limits: { cpu: "1", memory: 512Mi }
      volumes:
        - name: script
          configMap: { name: ${CM}, defaultMode: 0555 }
YAML

echo "==> applied Job ${NS}/${JOB}; waiting (max 25m) ..."
# Wait for either completion or failure; kubectl wait can't OR the two, so poll.
deadline=$(( $(date +%s) + 1500 ))
status=""
while [ "$(date +%s)" -lt "$deadline" ]; do
  if kubectl -n "$NS" get job "$JOB" -o jsonpath='{.status.succeeded}' 2>/dev/null | grep -q 1; then status="succeeded"; break; fi
  if kubectl -n "$NS" get job "$JOB" -o jsonpath='{.status.failed}' 2>/dev/null | grep -q 1; then status="failed"; break; fi
  sleep 5
done

echo "---- job logs ----"
kubectl -n "$NS" logs "job/${JOB}" --tail=50 2>/dev/null || true
echo "------------------"

case "$status" in
  succeeded) echo "✔ backup complete for '$NS'";;
  failed)    echo "::error::backup Job failed for '$NS'"; exit 1;;
  *)         echo "::error::backup Job did not finish in time for '$NS'"; exit 1;;
esac
