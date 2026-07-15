#!/usr/bin/env bash
#
# Generate the Beacon SealedSecrets for one environment.
#
# Produces deploy/helm/beacon/sealed/<env>/beacon-secrets.sealed.yaml (and
# beacon-registry.sealed.yaml when registry creds are available). Those files are
# ENCRYPTED with the cluster's sealed-secrets public key — only the in-cluster
# controller's private key can decrypt them — so they are safe to commit. The
# chart renders them when secretsSource=sealed; the controller then materialises
# the plain `beacon-secrets` Secret that the pods consume.
#
# Usage:
#   deploy/scripts/seal-secrets.sh int              # generate/refresh the sealed files
#   deploy/scripts/seal-secrets.sh int --apply      # ...and kubectl apply them now
#   deploy/scripts/seal-secrets.sh int --rotate     # force NEW random values
#   deploy/scripts/seal-secrets.sh int --show-keys  # list which keys are set, no values
#
# Where the values come from:
#   BEACON_AI_API_KEY                  <- deploy/.env
#   REGISTRY_USERNAME/REGISTRY_PASSWORD<- deploy/.env (OPTIONAL — omit to reuse an
#                                         existing pull secret via pullSecretCreate=false)
#   everything else                    <- generated here, then remembered in
#                                         deploy/.secrets/<env>.env (gitignored)
#
# Why we remember the generated values: POSTGRES_PASSWORD must stay STABLE. Postgres
# bakes the password into its data directory on first init; re-sealing with a fresh
# password would leave the pod unable to authenticate against its own volume. The
# cache makes re-runs idempotent. --rotate deliberately overrides it (and then you
# must also reset the Postgres role/volume).
#
# Prerequisite: the sealed-secrets controller must be installed in the cluster.
#   helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
#   helm upgrade --install sealed-secrets sealed-secrets/sealed-secrets \
#     -n kube-system --set fullnameOverride=sealed-secrets-controller
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART_DIR="$REPO_ROOT/deploy/helm/beacon"
ENV_FILE="$REPO_ROOT/deploy/.env"
CACHE_DIR="$REPO_ROOT/deploy/.secrets"

# The controller's Service. kubeseal fetches the public cert from it.
CONTROLLER_NAME="${SEALED_SECRETS_CONTROLLER_NAME:-sealed-secrets-controller}"
CONTROLLER_NS="${SEALED_SECRETS_CONTROLLER_NAMESPACE:-kube-system}"

TARGET_ENV=""
APPLY=false
ROTATE=false
SHOW_KEYS=false
CERT_ARG=""
# The controller's PUBLIC sealing certificate, committed to the repo. Sealing only
# needs the public key, so with this present you can re-seal from anywhere —
# including a laptop that cannot reach the k3s1 API (it is operator-network only).
# Refresh with:  kubeseal --fetch-cert > deploy/sealed/sealing-cert.pem
DEFAULT_CERT="$REPO_ROOT/deploy/sealed/sealing-cert.pem"

die() {
  echo "error: $*" >&2
  exit 1
}
note() { echo "  $*"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    int | acc | test | prod) TARGET_ENV="$1"; shift ;;
    --apply) APPLY=true; shift ;;
    --rotate) ROTATE=true; shift ;;
    --show-keys) SHOW_KEYS=true; shift ;;
    --cert) CERT_ARG="${2:-}"; shift 2 ;;
    -h | --help) sed -n '2,32p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1 (expected one of: int acc test prod [--apply] [--rotate] [--show-keys])" ;;
  esac
done

[[ -n "$TARGET_ENV" ]] || die "which environment? e.g. $(basename "$0") int"
command -v kubectl >/dev/null || die "kubectl is required"
command -v kubeseal >/dev/null || die "kubeseal is required (brew install kubeseal)"
command -v openssl >/dev/null || die "openssl is required"

NAMESPACE="$TARGET_ENV" # one namespace per env, matching the deploy workflow
OUT_DIR="$CHART_DIR/sealed/$TARGET_ENV"
CACHE_FILE="$CACHE_DIR/$TARGET_ENV.env"

# ---- load optional inputs from deploy/.env ---------------------------------
# Only the keys we care about, so an unrelated line in .env can never leak in.
read_env() { # read_env KEY FILE -> value on stdout (empty if absent)
  local key="$1" file="$2"
  [[ -f "$file" ]] || return 0
  sed -n "s/^${key}=//p" "$file" | tail -n1 | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//"
}

BEACON_AI_API_KEY="$(read_env BEACON_AI_API_KEY "$ENV_FILE")"
REGISTRY_USERNAME="$(read_env REGISTRY_USERNAME "$ENV_FILE")"
REGISTRY_PASSWORD="$(read_env REGISTRY_PASSWORD "$ENV_FILE")"

# ---- generated values, cached so re-runs are idempotent --------------------
mkdir -p "$CACHE_DIR"
chmod 700 "$CACHE_DIR"
if [[ -f "$CACHE_FILE" && "$ROTATE" == "false" ]]; then
  # shellcheck disable=SC1090
  source "$CACHE_FILE"
  note "reusing generated values from deploy/.secrets/${TARGET_ENV}.env (--rotate to replace)"
else
  [[ "$ROTATE" == "true" ]] && note "--rotate: generating NEW values (Postgres role/volume must be reset to match!)"
  # JWT secrets: backend requires >= 32 BYTES (config.go).
  BEACON_JWT_ACCESS_SECRET="$(openssl rand -base64 48 | tr -d '\n')"
  BEACON_JWT_REFRESH_SECRET="$(openssl rand -base64 48 | tr -d '\n')"
  # Encryption key: must DECODE to exactly 32 bytes -> 64 hex chars.
  BEACON_ENCRYPTION_KEY="$(openssl rand -hex 32)"
  BEACON_WEBHOOK_TOKEN="$(openssl rand -hex 24)"
  # Postgres password is interpolated into a DSN (postgres://user:PASS@host), so it
  # must be URI-safe: alphanumerics only. A '@', '/', ':' or '#' would silently
  # corrupt the connection string.
  # Filter with bash parameter expansion rather than `tr </dev/urandom | head`:
  # head closes the pipe on that infinite stream, which SIGPIPEs tr and, under
  # `set -o pipefail`, kills the script (exit 141).
  _pg_raw="$(openssl rand -base64 64 | tr -d '\n')"
  _pg_alnum="${_pg_raw//[^A-Za-z0-9]/}"
  POSTGRES_PASSWORD="${_pg_alnum:0:32}"
  unset _pg_raw _pg_alnum

  umask 077
  cat >"$CACHE_FILE" <<EOF
# Generated by seal-secrets.sh for env=$TARGET_ENV. GITIGNORED — never commit.
# These are the plaintext behind sealed/$TARGET_ENV/*.sealed.yaml. Keep them: the
# sealed files cannot be decrypted locally, and POSTGRES_PASSWORD must stay stable.
BEACON_JWT_ACCESS_SECRET=$BEACON_JWT_ACCESS_SECRET
BEACON_JWT_REFRESH_SECRET=$BEACON_JWT_REFRESH_SECRET
BEACON_ENCRYPTION_KEY=$BEACON_ENCRYPTION_KEY
BEACON_WEBHOOK_TOKEN=$BEACON_WEBHOOK_TOKEN
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
EOF
  chmod 600 "$CACHE_FILE"
  note "generated new values -> deploy/.secrets/${TARGET_ENV}.env (gitignored, 0600)"
fi

# ---- sanity-check the values against the backend's own validation ----------
[[ ${#BEACON_JWT_ACCESS_SECRET} -ge 32 ]] || die "BEACON_JWT_ACCESS_SECRET must be >= 32 bytes"
[[ ${#BEACON_JWT_REFRESH_SECRET} -ge 32 ]] || die "BEACON_JWT_REFRESH_SECRET must be >= 32 bytes"
[[ ${#BEACON_ENCRYPTION_KEY} -eq 64 ]] || die "BEACON_ENCRYPTION_KEY must be 64 hex chars (32 bytes)"
[[ "$BEACON_ENCRYPTION_KEY" =~ ^[0-9a-fA-F]{64}$ ]] || die "BEACON_ENCRYPTION_KEY must be hex"
[[ "$POSTGRES_PASSWORD" =~ ^[A-Za-z0-9]+$ ]] || die "POSTGRES_PASSWORD must be alphanumeric (it goes into a DSN URI)"

if [[ "$SHOW_KEYS" == "true" ]]; then
  echo "Keys that will be sealed for env=$TARGET_ENV (values withheld):"
  for k in BEACON_JWT_ACCESS_SECRET BEACON_JWT_REFRESH_SECRET BEACON_ENCRYPTION_KEY \
    BEACON_WEBHOOK_TOKEN POSTGRES_PASSWORD BEACON_AI_API_KEY; do
    v="${!k:-}"
    printf '  %-28s %s\n' "$k" "$([[ -n "$v" ]] && echo "set (${#v} chars)" || echo "EMPTY")"
  done
  printf '  %-28s %s\n' "REGISTRY_USERNAME" "$([[ -n "$REGISTRY_USERNAME" ]] && echo "set" || echo "absent -> reuse existing pull secret")"
  exit 0
fi

# ---- resolve the sealing certificate ---------------------------------------
# Sealing is asymmetric: it needs only the controller's PUBLIC cert, never the
# cluster. Prefer an explicit --cert, then the committed cert, and only reach out
# to the cluster as a last resort (the k3s1 API is not reachable off-network).
CERT="$(mktemp)"
trap 'rm -f "$CERT"' EXIT
if [[ -n "$CERT_ARG" ]]; then
  [[ -s "$CERT_ARG" ]] || die "--cert $CERT_ARG is missing or empty"
  cp "$CERT_ARG" "$CERT"
  note "sealing with the cert at $CERT_ARG"
elif [[ -s "$DEFAULT_CERT" ]]; then
  cp "$DEFAULT_CERT" "$CERT"
  note "sealing with the committed cert (${DEFAULT_CERT#"$REPO_ROOT"/})"
else
  note "no committed cert — fetching from ${CONTROLLER_NS}/${CONTROLLER_NAME} (needs cluster access)"
  if ! kubeseal --controller-name "$CONTROLLER_NAME" --controller-namespace "$CONTROLLER_NS" \
    --fetch-cert >"$CERT" 2>/dev/null || [[ ! -s "$CERT" ]]; then
    die "could not fetch the sealing cert from ${CONTROLLER_NS}/${CONTROLLER_NAME}.
       The k3s1 API is only reachable from the operator network, so this will fail
       from a laptop. Either pass --cert <file>, or commit the cert:
         kubeseal --fetch-cert > deploy/sealed/sealing-cert.pem
       If the controller is not installed:
         kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.27.1/controller.yaml"
  fi
fi
openssl x509 -in "$CERT" -noout >/dev/null 2>&1 || die "the sealing cert is not a valid X.509 certificate"
note "cert OK (expires $(openssl x509 -in "$CERT" -noout -enddate 2>/dev/null | cut -d= -f2))"

mkdir -p "$OUT_DIR"

# seal NAME <kubectl-create-secret-args...>
# Builds the Secret client-side (never applied in plaintext) and pipes it straight
# into kubeseal, so the plaintext Secret never touches disk or the cluster.
seal() {
  local name="$1"
  shift
  local out="$OUT_DIR/${name}.sealed.yaml"
  kubectl create secret "$@" \
    --namespace "$NAMESPACE" \
    --dry-run=client -o yaml |
    kubeseal --format yaml --cert "$CERT" \
      --namespace "$NAMESPACE" --name "$name" >"$out.tmp"
  # kubeseal emits the CRD; make sure it actually produced encryptedData.
  grep -q "encryptedData" "$out.tmp" || {
    rm -f "$out.tmp"
    die "kubeseal produced no encryptedData for $name"
  }
  grep -qi "stringData\|^  [A-Z_]*: [A-Za-z0-9+/=]\{40,\}$" "$out.tmp" && true
  mv "$out.tmp" "$out"
  note "sealed -> ${out#"$REPO_ROOT"/}"
}

# ---- beacon-secrets --------------------------------------------------------
ARGS=(generic beacon-secrets
  --from-literal=BEACON_JWT_ACCESS_SECRET="$BEACON_JWT_ACCESS_SECRET"
  --from-literal=BEACON_JWT_REFRESH_SECRET="$BEACON_JWT_REFRESH_SECRET"
  --from-literal=BEACON_ENCRYPTION_KEY="$BEACON_ENCRYPTION_KEY"
  --from-literal=BEACON_WEBHOOK_TOKEN="$BEACON_WEBHOOK_TOKEN"
  --from-literal=POSTGRES_PASSWORD="$POSTGRES_PASSWORD")
# The AI key is optional — the chart marks it optional so an empty value never
# blocks pod start. Only include it when we actually have one.
if [[ -n "$BEACON_AI_API_KEY" ]]; then
  ARGS+=(--from-literal=BEACON_AI_API_KEY="$BEACON_AI_API_KEY")
  note "including BEACON_AI_API_KEY from deploy/.env"
else
  note "BEACON_AI_API_KEY absent from deploy/.env — omitting (AI enrichment degrades to no-analysis)"
fi
seal beacon-secrets "${ARGS[@]}"

# ---- beacon-registry (optional) --------------------------------------------
if [[ -n "$REGISTRY_USERNAME" && -n "$REGISTRY_PASSWORD" ]]; then
  REGISTRY_HOST="$(sed -n 's/^ *registry: *//p' "$CHART_DIR/values-${TARGET_ENV}.yaml" | tail -n1)"
  [[ -n "$REGISTRY_HOST" ]] || die "could not read image.registry from values-${TARGET_ENV}.yaml"
  note "sealing a pull secret for $REGISTRY_HOST"
  seal beacon-registry docker-registry beacon-registry \
    --docker-server="$REGISTRY_HOST" \
    --docker-username="$REGISTRY_USERNAME" \
    --docker-password="$REGISTRY_PASSWORD"
  echo
  echo "NOTE: set image.pullSecret=beacon-registry and image.pullSecretCreate=true"
  echo "      in values-${TARGET_ENV}.yaml to use this sealed pull secret."
else
  note "no REGISTRY_USERNAME/REGISTRY_PASSWORD in deploy/.env — skipping the pull secret."
  note "  (values-${TARGET_ENV}.yaml can reference an existing one via pullSecretCreate=false)"
fi

# ---- apply -----------------------------------------------------------------
if [[ "$APPLY" == "true" ]]; then
  echo
  echo "Applying to namespace '$NAMESPACE' ..."
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl apply -n "$NAMESPACE" -f "$OUT_DIR"
  echo
  echo "Waiting for the controller to unseal ..."
  for _ in $(seq 1 30); do
    if kubectl -n "$NAMESPACE" get secret beacon-secrets >/dev/null 2>&1; then
      echo "  beacon-secrets materialised."
      break
    fi
    sleep 2
  done
  kubectl -n "$NAMESPACE" get sealedsecret 2>/dev/null || true
fi

echo
echo "============================================"
echo "  Sealed secrets for '$TARGET_ENV' -> $(ls "$OUT_DIR" | tr '\n' ' ')"
echo "  These files are ENCRYPTED and safe to commit."
echo "  Plaintext lives ONLY in deploy/.secrets/${TARGET_ENV}.env (gitignored)."
echo "============================================"
