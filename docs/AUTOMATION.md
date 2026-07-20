# Managing monitors from CI

Keep the domains you monitor next to the code that serves them, and keep the two in
step automatically.

This guide is for the person wiring Beacon Pulse into a pipeline. For the full endpoint
list, see [API.md](API.md).

---

## 1. Create an API key

Dashboard → **API keys** → *Create key*. Name it after whatever will use it
(`github-actions`), and give it the least access it needs — `viewer` if it only reads.

The secret is shown **once**. Only a hash is stored, so it cannot be shown again or
recovered: copy it straight into your secret store. If it is lost, revoke it and make
another.

```
Settings → Secrets and variables → Actions → New repository secret
  Name:   BEACON_API_KEY
  Value:  bp_xxxxxxxxxxxxxxxxxxxxxxxx
```

### What a key is, and is not

- **It belongs to the organization, not to you.** It keeps working after you leave, and
  its changes are attributed to the org. Revoke it when the thing using it is retired.
- **It carries no plan or balance.** It resolves to your organization, and your plan,
  credit and limits are read live on every request — so upgrading, downgrading or
  topping up takes effect immediately, with no new key.
- **It cannot create other keys.** That requires signing in, so a leaked key cannot mint
  itself a successor that outlives your revoking it.

---

## 2. Declare your monitors

Commit a file describing what you want monitored:

**`monitors.json`**

```json
{
  "project": "production",
  "monitors": [
    { "name": "www", "type": "https", "target": "https://example.com", "public": true },

    { "name": "api", "type": "https", "target": "https://api.example.com",
      "interval_seconds": 30,
      "settings": {
        "valid_status_codes": [200],
        "body_keyword": "ok",
        "response_time_warning_ms": 2000,
        "ssl_expiry_warning_days": 14
      } },

    { "name": "db", "type": "tcp", "target": "db.example.com:5432" },

    { "name": "nightly-backup", "type": "heartbeat", "grace_seconds": 3600 }
  ]
}
```

`name` is the identity. It is how a monitor is matched from one run to the next, so
renaming one creates a new monitor and reports the old as removable.

---

## 3. Apply it on push

**`.github/workflows/monitors.yml`**

```yaml
name: Sync monitors

on:
  push:
    branches: [main]
    paths: ['monitors.json']
  pull_request:
    paths: ['monitors.json']

env:
  BEACON_URL: https://beaconpulse.net

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # On a pull request, show what WOULD change and apply none of it, so a
      # reviewer sees the plan before it is merged.
      - name: Plan
        if: github.event_name == 'pull_request'
        run: |
          jq '. + {dry_run: true}' monitors.json > payload.json
          curl -sS --fail-with-body -X POST "$BEACON_URL/api/v1/sync" \
            -H "Authorization: Bearer $BEACON_API_KEY" \
            -H "Content-Type: application/json" \
            -d @payload.json > result.json
          jq -r '.items[] | "  \(.action)\t\(.name)"' result.json
        env:
          BEACON_API_KEY: ${{ secrets.BEACON_API_KEY }}

      - name: Apply
        if: github.event_name == 'push'
        run: |
          curl -sS --fail-with-body -X POST "$BEACON_URL/api/v1/sync" \
            -H "Authorization: Bearer $BEACON_API_KEY" \
            -H "Content-Type: application/json" \
            -d @monitors.json > result.json

          jq -r '.items[] | "  \(.action)\t\(.name)"' result.json

          # A monitor rejected on its own merits (a plan limit, a bad target) still
          # returns 200, because the rest of the file WAS applied. So the status code
          # alone would let it through silently — check the count.
          failed=$(jq -r '.failed' result.json)
          if [ "$failed" != "0" ]; then
            jq -r '.items[] | select(.action=="error") | "::error::\(.name): \(.error)"' result.json
            exit 1
          fi
        env:
          BEACON_API_KEY: ${{ secrets.BEACON_API_KEY }}
```

That is the whole integration. Push a change to `monitors.json` and your monitoring
follows it.

---

## Why it is safe to run on every push

`POST /api/v1/sync` is **idempotent**: it takes the set you want and works out the
difference. Run it a hundred times with an unchanged file and it makes zero writes and
reports everything `unchanged`.

This is the reason not to build the obvious thing. A workflow that called
`POST /api/v1/monitors` per domain would create a duplicate on every re-run, retry and
re-merge — and a fortnight later the same domain is probed forty times, and billed
forty times.

### Deleting is opt-in

Remove a monitor from the file and it is **reported, not deleted**:

```json
{ "name": "old-api", "action": "would_remove", "id": "..." }
```

Add `"prune": true` to actually remove it.

That default is deliberate, and worth understanding before you turn it off. A workflow
with a bad path filter, an empty matrix, or a failed template step declares *zero*
monitors. With pruning on by default, that one mistake silently deletes your production
monitoring — at the exact moment nobody is watching, because the monitoring is gone.
Off, the same mistake changes nothing and tells you what it would have done.

Turn it on when you trust the pipeline, and the file becomes the single source of
truth: delete a line, and the monitor goes with it.

---

## Reading the response

```json
{
  "project": "production",
  "dry_run": false,
  "created": 1, "updated": 1, "unchanged": 2,
  "removed": 0, "would_remove": 1, "failed": 0,
  "items": [
    { "name": "api",     "action": "updated",      "id": "..." },
    { "name": "db",      "action": "unchanged",    "id": "..." },
    { "name": "old-api", "action": "would_remove", "id": "..." },
    { "name": "www",     "action": "created",      "id": "..." }
  ]
}
```

| action | meaning |
|---|---|
| `created` | did not exist; now does |
| `updated` | existed and differed; now matches the file |
| `unchanged` | already matched — no write was performed |
| `would_remove` | no longer declared; pass `prune` to remove it |
| `removed` | deleted, because `prune` was set |
| `error` | this one failed; the others still applied |

Items are sorted by name, so consecutive runs diff cleanly.

---

## Other things you can automate

Any endpoint the dashboard uses accepts an API key:

```bash
# What is failing right now
curl -H "Authorization: Bearer $BEACON_API_KEY" \
  "$BEACON_URL/api/v1/monitors?status=down"

# How close to your plan's limit you are
curl -H "Authorization: Bearer $BEACON_API_KEY" \
  "$BEACON_URL/api/v1/monitors/usage"

# Pause a monitor during a deploy, then resume it
curl -X POST -H "Authorization: Bearer $BEACON_API_KEY" \
  "$BEACON_URL/api/v1/monitors/$ID/pause"
```

For a planned deploy, prefer a **maintenance window** over pausing — alerts are
suppressed but the monitor keeps recording, so you do not lose the history of what
happened during it.

---

## Troubleshooting

**`401 invalid API key`** — revoked, expired, or mistyped. Every failure returns the
same message on purpose, so a stolen key learns nothing from the response; check the
key's status in the dashboard.

**`403`** — the key's role is too low. A `viewer` key cannot write.

**`422 monitor limit reached`** — the org is at its plan's limit. The other monitors in
the file still applied; this one is in `items` with `action: "error"`.

**Everything reports `created` on the second run** — the `name` changed between runs.
Name is the identity; a renamed monitor is a new one.
