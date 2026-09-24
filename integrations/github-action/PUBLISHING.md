# Publishing the Beacon Notify action

> This file is a guide for maintainers. It does **not** need to live in the published
> action repo — copy it or leave it out, either is fine.

The dashboard shows users this snippet:

```yaml
- uses: bwalia/beacon-notify@v1
```

For that to resolve, the contents of this folder must live at the **root** of a
public repo `bwalia/beacon-notify`, tagged `v1`. Steps:

## 1. Create the repo and copy the files

```sh
# from the monorepo root
gh repo create bwalia/beacon-notify --public \
  --description "Report GitHub Actions workflow results to Beacon / SysOps 24/7"

tmp="$(mktemp -d)"
git -C "$tmp" clone git@github.com:bwalia/beacon-notify.git .
cp -R integrations/github-action/. "$tmp"/
rm -f "$tmp"/PUBLISHING.md              # optional: keep it out of the action repo
git -C "$tmp" add -A
git -C "$tmp" commit -m "feat: initial Beacon Notify action (v1.0.0)"
git -C "$tmp" push origin main
```

`action.yml` must end up at the **repo root** — GitHub only lists an action whose
`action.yml` (or `action.yaml`) is at the root of the repo (or of a tagged subdir).

## 2. Tag and publish to the Marketplace

```sh
git -C "$tmp" tag -a v1.0.0 -m "Beacon Notify v1.0.0"
git -C "$tmp" push origin v1.0.0
gh release create v1.0.0 --repo bwalia/beacon-notify \
  --title "v1.0.0" --notes-file "$tmp/CHANGELOG.md"
```

Then, in the GitHub UI for that release, tick **"Publish this Action to the GitHub
Marketplace"**, choose the **Monitoring** category, and accept the terms. (You can
also do this while drafting the release the first time.)

## 3. Add the moving major tag

So `@v1` tracks the latest `1.x`:

```sh
git -C "$tmp" tag -f v1 v1.0.0
git -C "$tmp" push -f origin v1
```

Re-run this (retagging `v1` to each new `v1.x.y`) on every future patch/minor.

## Notes

- **Name collisions:** the Marketplace requires a unique action name. If "Beacon
  Notify" is taken, change `name:` in `action.yml` before publishing.
- **Different owner/name:** if you publish under anything other than
  `bwalia/beacon-notify`, update the snippet in the dashboard
  (`frontend/src/app/(dashboard)/monitors/page.tsx`, the `GitHubIngestReveal`
  component) and in `README.md` to match.
- **No Marketplace required:** even unpublished, `uses: bwalia/beacon-notify@v1`
  works as long as the repo is public and the `v1` tag exists. The Marketplace
  listing is discoverability, not a functional requirement.
