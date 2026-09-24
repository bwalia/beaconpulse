#!/usr/bin/env bash
#
# notify.sh — the body of the "Beacon Notify" composite action.
#
# Builds a small JSON document from the run's GITHUB_* context and POSTs it to the
# Beacon ingest URL. Beacon decides from `status` whether to alert (a failed run) or
# clear (a success after a failure) and fans it out to the org's channels.
#
# Two rules this file lives by:
#   1. Never fail the caller's build. A monitoring hook that breaks CI is worse than
#      no hook, so every error path exits 0.
#   2. Escape values safely. Commit branches and workflow names are user-controlled,
#      so JSON is assembled with jq, never string interpolation.
set -uo pipefail

if [ -z "${BEACON_URL:-}" ]; then
  echo "beacon-notify: no url provided (set the 'url' input); skipping." >&2
  exit 0
fi

for bin in curl jq; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "beacon-notify: '$bin' not found on this runner; skipping. (Present on all GitHub-hosted runners.)" >&2
    exit 0
  fi
done

STATUS="${BEACON_STATUS:-}"
RUN_URL="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-}/actions/runs/${GITHUB_RUN_ID:-}"

payload="$(jq -n \
  --arg status     "$STATUS" \
  --arg repository "${GITHUB_REPOSITORY:-}" \
  --arg workflow   "${GITHUB_WORKFLOW:-}" \
  --arg run_id     "${GITHUB_RUN_ID:-}" \
  --arg run_number "${GITHUB_RUN_NUMBER:-}" \
  --arg run_attempt "${GITHUB_RUN_ATTEMPT:-1}" \
  --arg branch     "${GITHUB_REF_NAME:-}" \
  --arg sha        "${GITHUB_SHA:-}" \
  --arg actor      "${GITHUB_ACTOR:-}" \
  --arg event      "${GITHUB_EVENT_NAME:-}" \
  --arg run_url    "$RUN_URL" \
  '{status:$status, repository:$repository, workflow:$workflow, run_id:$run_id,
    run_number:$run_number, run_attempt:$run_attempt, branch:$branch, sha:$sha,
    actor:$actor, event:$event, run_url:$run_url}')"

code="$(curl -sS -m 15 -o /dev/null -w '%{http_code}' \
  -X POST "$BEACON_URL" \
  -H 'Content-Type: application/json' \
  -d "$payload" 2>/dev/null)" || {
  echo "beacon-notify: request failed (network); not failing the build." >&2
  exit 0
}

if [ "$code" = "200" ]; then
  echo "beacon-notify: reported '${STATUS:-unknown}' to Beacon."
else
  echo "beacon-notify: Beacon returned HTTP $code (check the url secret); not failing the build." >&2
fi
exit 0
