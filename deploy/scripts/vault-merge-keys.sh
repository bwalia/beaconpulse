#!/usr/bin/env bash
#
# vault-merge-keys.sh — MERGE specific keys into one env's wslvault config,
# WITHOUT rewriting the rest of the secret.
#
# Why this exists: vault-load-secrets.sh rebuilds the WHOLE beaconpulse/<env>/config
# object from local files, so any key present in Vault but absent from those files
# (e.g. STRIPE_* on prod, which live in Vault out-of-band and are emptied in
# deploy/.env.<env>) gets dropped. This tool reads the current secret (or a chosen
# prior version), overlays only the named keys (values from deploy/.env + overlay),
# and writes a new version — nothing else is touched.
#
# Usage:
#   VAULT_ADDR=https://vault.workstation.co.uk VAULT_TOKEN=<jwt> \
#     deploy/scripts/vault-merge-keys.sh <env> [--from-version N] KEY [KEY ...]
#
# If VAULT_TOKEN is unset, the token is read from the ESO store secret
# (int/wslvault-token) via kubectl — the same credential ESO uses for this mount.
# --from-version N reads the base object from version N instead of current (used to
# recover after a bad full-replace: read the last-good version, re-add your keys).
set -euo pipefail

ENV="${1:-}"; shift || true
[ -n "$ENV" ] || { echo "usage: $0 <env> [--from-version N] KEY [KEY ...]" >&2; exit 2; }

FROM_VERSION=""
if [ "${1:-}" = "--from-version" ]; then FROM_VERSION="${2:?--from-version needs a number}"; shift 2; fi
[ "$#" -ge 1 ] || { echo "error: name at least one KEY to merge" >&2; exit 2; }
KEYS=("$@")

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADDR="${VAULT_ADDR:-https://vault.workstation.co.uk}"
ENVF="$REPO/deploy/.env"; OVL="$REPO/deploy/.env.${ENV}"
command -v jq >/dev/null || { echo "jq required" >&2; exit 1; }
command -v curl >/dev/null || { echo "curl required" >&2; exit 1; }

# Token: explicit env wins; otherwise read the ESO store credential from the cluster.
TOKEN="${VAULT_TOKEN:-}"
if [ -z "$TOKEN" ]; then
  command -v kubectl >/dev/null || { echo "set VAULT_TOKEN or have kubectl access to int/wslvault-token" >&2; exit 1; }
  TOKEN="$(kubectl -n int get secret wslvault-token -o jsonpath='{.data.token}' | base64 -d)"
fi
[ -n "$TOKEN" ] || { echo "empty vault token" >&2; exit 1; }

# read_env KEY: overlay (deploy/.env.<env>) wins over base (deploy/.env). Presence in
# the overlay is authoritative even when empty — mirrors seal/load scripts.
read_env() {
  local k="$1"
  if [ -f "$OVL" ] && grep -qE "^${k}=" "$OVL"; then sed -n "s/^${k}=//p" "$OVL" | tail -1; return; fi
  [ -f "$ENVF" ] && sed -n "s/^${k}=//p" "$ENVF" | tail -1
}

path="beaconpulse/${ENV}/config"
get_url="$ADDR/v1/kv/data/$path"; [ -n "$FROM_VERSION" ] && get_url="$get_url?version=$FROM_VERSION"

# 1) base object (current version, or the requested prior version)
base="$(curl -sf -H "X-Vault-Token: $TOKEN" "$get_url" | jq -c '.data.data')"
[ -n "$base" ] && [ "$base" != "null" ] || { echo "could not read $path${FROM_VERSION:+ (version $FROM_VERSION)}" >&2; exit 1; }

# 2) overlay only the named keys (skip any that resolve empty — never blank a real key)
merged="$base"
for k in "${KEYS[@]}"; do
  v="$(read_env "$k")"
  if [ -n "$v" ]; then
    merged="$(jq -c --arg k "$k" --arg v "$v" '. + {($k):$v}' <<<"$merged")"
    echo "  + $k"
  else
    echo "  - $k (empty in deploy/.env${OVL:+/.env.$ENV} — skipped)"
  fi
done

echo "==> merging $(printf '%s ' "${KEYS[@]}")into $path (base=$( [ -n "$FROM_VERSION" ] && echo "v$FROM_VERSION" || echo current ), total keys $(jq 'keys|length' <<<"$merged"))"
code=$(curl -s -o /tmp/vault-merge.out -w '%{http_code}' -X POST \
  -H "X-Vault-Token: $TOKEN" -H "Content-Type: application/json" \
  --data "$(jq -nc --argjson d "$merged" '{data:$d}')" "$ADDR/v1/kv/data/$path")
if [ "$code" = 200 ] || [ "$code" = 204 ]; then
  echo "✔ wrote $path (v$(jq -r '.data.version // "?"' /tmp/vault-merge.out 2>/dev/null))"
else
  echo "::error::write failed HTTP $code:"; cat /tmp/vault-merge.out; exit 1
fi
rm -f /tmp/vault-merge.out