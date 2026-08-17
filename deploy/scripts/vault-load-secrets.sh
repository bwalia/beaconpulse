#!/usr/bin/env bash
#
# vault-load-secrets.sh — push one env's Beacon secrets into wslvault's KV v2 mount,
# so External Secrets Operator (secretsSource: external) can sync them into the
# cluster. Replaces kubeseal for production.
#
# Writes a single JSON object to kv/beaconpulse/<env>/config — exactly the keys the
# API/worker read (same set the sealed secret carried). ESO's dataFrom.extract then
# pulls them all into the beacon-secrets Secret.
#
# Values come from the same place seal-secrets.sh uses:
#   deploy/.secrets/<env>.env  — the stable generated values (jwt/encryption/pg)
#   deploy/.env (+ .env.<env>)  — AI key, Google client id, Stripe
#
# Usage:
#   VAULT_ADDR=https://vault.workstation.co.uk VAULT_TOKEN=<wslvault-jwt> \
#     deploy/scripts/vault-load-secrets.sh prod
#
# Auth: X-Vault-Token (a wslvault JWT). For header-auth deployments, set instead
#   VAULT_TENANT_ID + VAULT_PRINCIPAL_ID + VAULT_POLICIES.
set -euo pipefail

ENV="${1:-}"; [ -n "$ENV" ] || { echo "usage: $0 <env> (prod|sysops-prod|…)" >&2; exit 2; }
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADDR="${VAULT_ADDR:?set VAULT_ADDR (e.g. https://vault.workstation.co.uk)}"
CACHE="$REPO/deploy/.secrets/${ENV}.env"
ENVF="$REPO/deploy/.env"; OVL="$REPO/deploy/.env.${ENV}"
command -v jq >/dev/null || { echo "jq required" >&2; exit 1; }

# read_env KEY: overlay wins over base, quotes stripped (mirrors seal-secrets.sh).
read_env() {
  local k="$1" v=""
  [ -f "$OVL" ] && grep -qE "^${k}=" "$OVL" && { sed -n "s/^${k}=//p" "$OVL" | tail -1; return; }
  [ -f "$ENVF" ] && sed -n "s/^${k}=//p" "$ENVF" | tail -1
}

# Generated values (stable) from the cache; fail loudly if absent — never ship blanks.
[ -f "$CACHE" ] || { echo "::error::missing $CACHE — generate it first (deploy/scripts/seal-secrets.sh $ENV) or copy it in"; exit 1; }
# shellcheck disable=SC1090
. "$CACHE"

# Build the JSON map: required generated keys + optional inputs (only if non-empty).
obj=$(jq -n \
  --arg a "${BEACON_JWT_ACCESS_SECRET:?}" --arg r "${BEACON_JWT_REFRESH_SECRET:?}" \
  --arg e "${BEACON_ENCRYPTION_KEY:?}" --arg w "${BEACON_WEBHOOK_TOKEN:?}" \
  --arg p "${POSTGRES_PASSWORD:?}" \
  '{BEACON_JWT_ACCESS_SECRET:$a, BEACON_JWT_REFRESH_SECRET:$r, BEACON_ENCRYPTION_KEY:$e, BEACON_WEBHOOK_TOKEN:$w, POSTGRES_PASSWORD:$p}')
for k in BEACON_AI_API_KEY BEACON_GOOGLE_CLIENT_ID STRIPE_SECRET_KEY STRIPE_PUBLISHABLE_KEY STRIPE_WEBHOOK_SECRET STRIPE_PRICE_STARTER STRIPE_PRICE_PRO; do
  v="$(read_env "$k")"; [ -n "$v" ] && obj=$(jq --arg k "$k" --arg v "$v" '. + {($k):$v}' <<<"$obj")
done

# Auth headers: token, or direct tenant/principal/policies.
hdr=(-H "Content-Type: application/json")
if [ -n "${VAULT_TOKEN:-}" ]; then hdr+=(-H "X-Vault-Token: ${VAULT_TOKEN}")
elif [ -n "${VAULT_TENANT_ID:-}" ]; then
  hdr+=(-H "X-Tenant-Id: ${VAULT_TENANT_ID}" -H "X-Principal-Id: ${VAULT_PRINCIPAL_ID:-beacon-loader}" -H "X-Policies: ${VAULT_POLICIES:-admin}")
else echo "::error::set VAULT_TOKEN (a wslvault JWT) or VAULT_TENANT_ID/PRINCIPAL_ID/POLICIES" >&2; exit 1; fi

path="beaconpulse/${ENV}/config"
echo "==> writing $(jq -r 'keys|join(", ")' <<<"$obj") to ${ADDR}/v1/kv/data/${path}"
code=$(curl -s -o /tmp/vault-load.out -w '%{http_code}' -X POST "${hdr[@]}" \
  --data "$(jq -nc --argjson d "$obj" '{data:$d}')" \
  "${ADDR}/v1/kv/data/${path}")
if [ "$code" = 200 ] || [ "$code" = 204 ]; then
  echo "✔ loaded kv/${path} ($(jq -r '.version // "?"' /tmp/vault-load.out 2>/dev/null | sed 's/^/v/'))"
else
  echo "::error::write failed HTTP $code:"; cat /tmp/vault-load.out; exit 1
fi
rm -f /tmp/vault-load.out
