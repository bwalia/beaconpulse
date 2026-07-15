#!/usr/bin/env bash
# Create (once, idempotently) the recurring Stripe Prices the Starter/Pro
# subscriptions use, and print their price ids. Safe to re-run: each price is
# keyed by a stable lookup_key, so a second run reuses the existing price rather
# than creating a duplicate.
#
# Usage:
#   deploy/scripts/create-stripe-prices.sh            # reads STRIPE_SECRET_KEY from deploy/.env
#   STRIPE_SECRET_KEY=sk_live_... deploy/scripts/create-stripe-prices.sh
#   deploy/scripts/create-stripe-prices.sh --write    # also upsert the ids into deploy/.env
#
# The Stripe key never leaves this shell; only the resulting price_ ids are printed.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$REPO_ROOT/deploy/.env"
WRITE=false
[[ "${1:-}" == "--write" ]] && WRITE=true

# Resolve the secret key: env var wins, else read from deploy/.env.
if [[ -z "${STRIPE_SECRET_KEY:-}" && -f "$ENV_FILE" ]]; then
  STRIPE_SECRET_KEY="$(sed -n 's/^STRIPE_SECRET_KEY=//p' "$ENV_FILE" | tail -n1 | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")"
fi
[[ -n "${STRIPE_SECRET_KEY:-}" ]] || { echo "error: STRIPE_SECRET_KEY not set (env or deploy/.env)" >&2; exit 1; }

case "$STRIPE_SECRET_KEY" in
  sk_test_*|rk_test_*) MODE=test ;;
  sk_live_*|rk_live_*) MODE=live ;;
  *) MODE=unknown ;;
esac
echo "Stripe mode: $MODE"

# jsonget <json> <python-expr-on-d>  — parse a field without needing jq.
jsonget() { python3 -c 'import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))' "$1"; }

# ensure_price <display-name> <lookup_key> <amount_cents> -> prints the price id.
ensure_price() {
  local name="$1" key="$2" cents="$3" existing created
  existing="$(curl -sS -G https://api.stripe.com/v1/prices \
    -u "$STRIPE_SECRET_KEY:" \
    --data-urlencode "lookup_keys[]=$key" -d 'active=true' -d 'limit=1' \
    | jsonget "d['data'][0]['id'] if d.get('data') else ''")"
  if [[ -n "$existing" ]]; then
    echo "  reused $name -> $existing" >&2
    printf '%s' "$existing"
    return
  fi
  created="$(curl -sS https://api.stripe.com/v1/prices \
    -u "$STRIPE_SECRET_KEY:" \
    -d currency=usd \
    -d "unit_amount=$cents" \
    -d 'recurring[interval]=month' \
    -d "product_data[name]=$name" \
    -d "lookup_key=$key")"
  local id
  id="$(printf '%s' "$created" | jsonget "d.get('id','')")"
  [[ -n "$id" ]] || { echo "error: create failed for $name:" >&2; printf '%s\n' "$created" >&2; exit 1; }
  echo "  created $name -> $id" >&2
  printf '%s' "$id"
}

STARTER="$(ensure_price 'Beacon Pulse Starter' beacon_starter_monthly 1900)"
PRO="$(ensure_price 'Beacon Pulse Pro' beacon_pro_monthly 7900)"

echo
echo "STRIPE_PRICE_STARTER=$STARTER"
echo "STRIPE_PRICE_PRO=$PRO"

if [[ "$WRITE" == true ]]; then
  # Upsert both keys into deploy/.env (replace the line if present, else append).
  upsert() {
    local k="$1" v="$2"
    if grep -qE "^$k=" "$ENV_FILE" 2>/dev/null; then
      # portable in-place edit (BSD + GNU sed)
      sed -i.bak -e "s|^$k=.*|$k=$v|" "$ENV_FILE" && rm -f "$ENV_FILE.bak"
    else
      # Guarantee the file ends with a newline before appending, else the new
      # KEY=VALUE concatenates onto the last line and silently corrupts it.
      [[ -s "$ENV_FILE" && -n "$(tail -c1 "$ENV_FILE")" ]] && printf '\n' >> "$ENV_FILE"
      printf '%s=%s\n' "$k" "$v" >> "$ENV_FILE"
    fi
  }
  upsert STRIPE_PRICE_STARTER "$STARTER"
  upsert STRIPE_PRICE_PRO "$PRO"
  echo
  echo "Wrote both to deploy/.env. Next: deploy/scripts/seal-secrets.sh <env> && commit the sealed file."
fi
