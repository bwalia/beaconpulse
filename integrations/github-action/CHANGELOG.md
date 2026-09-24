# Changelog

All notable changes to the Beacon Notify action are documented here. This project
follows [Semantic Versioning](https://semver.org): the moving `v1` tag always points
at the latest `1.x` release.

## v1.0.0

Initial release.

- Composite action that POSTs a workflow run's result (status, repo, workflow, run
  number/attempt, branch, commit, actor, event, run URL) to a Beacon ingest URL.
- Reports both failures and recoveries when used with `if: ${{ always() }}` and
  `status: ${{ job.status }}`.
- Never fails the caller's build: any missing tool, network error, or non-200
  response is logged and swallowed (exit 0).
- Requires only `curl` and `jq` (present on all GitHub-hosted runners).
