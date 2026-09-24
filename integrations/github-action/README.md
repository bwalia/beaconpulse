# Beacon Notify — GitHub Action

Report your GitHub Actions workflow results to **Beacon / SysOps 24/7** and get
alerted through your existing channels (Telegram, Slack, email, iOS push) the moment
a run fails — no polling, no GitHub token, no rate limits.

Beacon never calls the GitHub API. This action *pushes* each run's result to a
capability URL that belongs to one monitor in your Beacon organization.

## Setup (once per repo)

1. In Beacon, create a **GitHub Actions** monitor for your repo. It shows you an
   **ingest URL** exactly once.
2. In GitHub, add that URL as a repository secret named `BEACON_NOTIFY_URL`
   (**Settings → Secrets and variables → Actions → New repository secret**).
3. Add the step below to the end of the job you want watched.

## Usage

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      # ... your build/test steps ...

      - name: Notify Beacon
        if: ${{ always() }}          # report both failures and recoveries
        uses: bwalia/beacon-notify@v1
        with:
          url: ${{ secrets.BEACON_NOTIFY_URL }}
          status: ${{ job.status }}
```

- Use `if: ${{ always() }}` (recommended) so a later green run clears the alert.
- Use `if: ${{ failure() }}` instead if you only ever want failure alerts (you can
  then drop the `status` input).

### Inputs

| Input    | Required | Default            | Description                                                        |
| -------- | -------- | ------------------ | ------------------------------------------------------------------ |
| `url`    | yes      | —                  | Your Beacon ingest URL. Keep it in a secret.                       |
| `status` | no       | `${{ job.status }}`| The result to report: `success`, `failure`, or `cancelled`.        |

Beacon alerts on `failure` (and `timed_out`), clears on `success` after a failure,
and ignores `cancelled`/`skipped`. If your monitor sets a **workflow filter**, only
runs of that workflow act on it.

## Without the action (plain curl)

The action is a thin wrapper; a `curl` step does the same and needs nothing but the
tools already on every hosted runner:

```yaml
      - name: Notify Beacon
        if: ${{ always() }}
        run: |
          curl -sS -m 15 -X POST "${{ secrets.BEACON_NOTIFY_URL }}" \
            -H 'Content-Type: application/json' \
            -d "$(jq -n \
              --arg status "${{ job.status }}" \
              --arg repository "$GITHUB_REPOSITORY" \
              --arg workflow "$GITHUB_WORKFLOW" \
              --arg run_id "$GITHUB_RUN_ID" \
              --arg branch "$GITHUB_REF_NAME" \
              --arg run_url "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID" \
              '{status:$status,repository:$repository,workflow:$workflow,run_id:$run_id,branch:$branch,run_url:$run_url}')" \
          || true   # never fail the build on a notification error
```

## Security

- The ingest URL is a **capability credential** — anyone who has it can post run
  results for that one monitor. Keep it in a repository secret and rotate it from the
  monitor's **Edit** dialog if it may have leaked. Beacon stores only a hash of the
  token, never the token itself.
- The action never fails your build: any network or configuration error is logged and
  swallowed.

## Requirements

`curl` and `jq` — both preinstalled on all GitHub-hosted runners. On a self-hosted
runner without them, install them or use a Docker-based step.
