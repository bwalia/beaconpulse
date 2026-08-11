# Beacon secrets — operator & agent runbook

How Beacon's secrets are provisioned, and the exact steps to (re)generate the
sealed secrets for an environment from **any machine** and push them to GitHub.

This file is written to be executed by a **Claude agent** as well as read by a
human. If you are an agent: read the [Agent task](#agent-task) section first, obey
the [Hard rules](#hard-rules), then follow [Runbook](#runbook) for the env you were
asked about. Do not improvise around the rules.

---

## The model in one line

Secrets are **encrypted with Bitnami SealedSecrets (`kubeseal`)** and committed to
git as ciphertext. Only the sealed-secrets controller's private key *inside the
cluster* can decrypt them, so the encrypted files are safe in the repo. Every base
environment already runs on this:

| env  | values file                          | mode              |
|------|--------------------------------------|-------------------|
| int  | `deploy/helm/beacon/values-int.yaml` | `secretsSource: sealed` |
| acc  | `deploy/helm/beacon/values-acc.yaml` | `secretsSource: sealed` |
| test | `deploy/helm/beacon/values-test.yaml`| `secretsSource: sealed` |
| prod | `deploy/helm/beacon/values-prod.yaml`| `secretsSource: sealed` |

The chart also supports `external` (Vault via External Secrets — **not used**, the
Vault policy was never granted), `plain` (local dev only, never commit values), and
`existing` (white-label brands copy a base env's secret — see the bottom section).
For go-live you only care about **`sealed`**.

---

## Trust boundary — what lives where

| Thing                         | Location                                                  | Committed?              | Plaintext? |
|-------------------------------|----------------------------------------------------------|-------------------------|------------|
| Encrypted secrets             | `deploy/helm/beacon/sealed/<env>/beacon-secrets.sealed.yaml` | ✅ yes               | ❌ ciphertext |
| Public sealing certificate    | `deploy/sealed/sealing-cert.pem`                         | ✅ yes (valid to 2036)  | ❌ public key |
| **Plaintext cache**           | `deploy/.secrets/<env>.env`                              | ❌ **gitignored, 0600** | ✅ **the real values** |
| Input secrets (AI/Stripe/Google) | `deploy/.env` (+ `deploy/.env.<env>` overlay)         | ❌ gitignored           | ✅ yes |

`.gitignore` enforces this: `.env*` and `deploy/.secrets/` are blocked; only
`!deploy/sealed/sealing-cert.pem` is whitelisted. **Never override that.**

---

## Keys that get sealed into `beacon-secrets`

| Key                          | Source                        | Notes |
|------------------------------|-------------------------------|-------|
| `BEACON_JWT_ACCESS_SECRET`   | **generated** (≥32 bytes)     | cached, stable |
| `BEACON_JWT_REFRESH_SECRET`  | **generated** (≥32 bytes)     | cached, stable |
| `BEACON_ENCRYPTION_KEY`      | **generated** (64 hex chars)  | cached, stable — **rotating orphans all encrypted data** |
| `BEACON_WEBHOOK_TOKEN`       | **generated**                 | cached, stable |
| `POSTGRES_PASSWORD`          | **generated** (alnum, 32)     | cached, stable — **rotating breaks the existing DB volume** |
| `BEACON_AI_API_KEY`          | `deploy/.env`                 | optional — absent ⇒ AI enrichment off |
| `BEACON_GOOGLE_CLIENT_ID`    | `deploy/.env`                 | optional — absent ⇒ Google sign-in off (backend side) |
| `STRIPE_*`                   | `deploy/.env`(+overlay)       | optional — present-but-empty in `.env.<env>` = billing OFF for that env |

The four **generated** values are written once to `deploy/.secrets/<env>.env` and
reused on every re-run so the output is idempotent. That cache is the crux of this
whole runbook — see next.

---

## The one thing that will bite you: the plaintext cache

`POSTGRES_PASSWORD` and `BEACON_ENCRYPTION_KEY` **must stay stable** across re-seals:

- Postgres bakes its password into its data directory on first init. Re-sealing an
  already-running env with a *new* password leaves the pod unable to authenticate
  against its own volume.
- The encryption key decrypts tenant secrets already stored in the DB. A new key
  makes all of that unreadable.

`seal-secrets.sh` keeps them stable by caching them in `deploy/.secrets/<env>.env`.
That file is **gitignored and never leaves the machine that generated it** unless
you deliberately copy it. Therefore:

- **Re-sealing an existing/live env from a fresh machine → you MUST bring that env's
  cache file first**, or you will generate brand-new values and break the running
  database. This is [Case A](#case-a--re-seal-an-existing-live-env).
- **A brand-new env that was never sealed** has no cache and none is needed; the
  script generates fresh values and *writes* the cache. Back it up immediately —
  it is the only copy of the plaintext. This is [Case B](#case-b--brand-new-env).

---

## Agent task

You have been dropped on a server with this repo checked out and told to
(re)generate the sealed secrets for one or more environments and push them.

Decision tree:

1. **Identify the env(s)** you were asked about (`int` / `acc` / `test` / `prod`).
2. For each env, does `deploy/.secrets/<env>.env` exist on this machine?
   - **Yes** → the stable values are present. Proceed with [Case A](#case-a--re-seal-an-existing-live-env).
   - **No, but the env is already live** (a `sealed/<env>/beacon-secrets.sealed.yaml`
     is already committed and the cluster is running it) → **STOP and ask the human
     for the cache file.** Do not generate fresh values for a live env. Sealing will
     silently produce a working-looking file that breaks Postgres on deploy.
   - **No, and it's a genuinely new env** → [Case B](#case-b--brand-new-env).
3. Run the seal command, verify, commit the **allowlisted files only**, push.
4. Report back: which env, which files changed, and the `git status` before push.

### Hard rules

- **NEVER** print, echo, `cat`, or paste the contents of `deploy/.env`,
  `deploy/.env.*`, or `deploy/.secrets/*`. Use `--show-keys` if you need to confirm
  which keys are set — it prints names and "set/EMPTY" only.
- **NEVER** `git add` anything matching `.env`, `deploy/.secrets/`, or any plaintext
  secret. Only the files in the [commit allowlist](#what-to-commit) may be staged.
- **NEVER** run `--rotate` unless a human explicitly asked to rotate AND confirmed
  they will reset the Postgres role/volume for that env.
- **NEVER** regenerate secrets for a live env without its cache file (see step 2).
- Before `git commit`, run `git status` and confirm every staged path is on the
  allowlist. If anything else is staged, `git restore --staged` it and stop.
- If a required tool or the cache is missing, stop and report — do not work around it.

---

## Prerequisites

```bash
command -v kubectl kubeseal openssl   # all three must resolve
# macOS:  brew install kubeseal
# Linux:  see https://github.com/bitnami-labs/sealed-secrets/releases
```

You do **not** need cluster access to seal. The public sealing cert is committed at
`deploy/sealed/sealing-cert.pem` (valid to 2036) and the script uses it offline —
which matters because the k3s1 API is only reachable from the operator network.

---

## Runbook

### Case A — re-seal an existing/live env

Example: `prod`. Substitute your env everywhere.

```bash
cd <repo-root>

# 1. Put the env's plaintext cache in place (copy it securely from the machine that
#    last sealed this env — scp/age, NOT git/Slack/email). Result:
#      deploy/.secrets/prod.env      (0600)
#    Confirm it's there WITHOUT printing it:
test -f deploy/.secrets/prod.env && echo "cache present" || echo "MISSING — stop, get the cache"

# 2. Provide the input secrets in deploy/.env (AI key, Google client id, Stripe).
#    Per-env values (e.g. prod's own Stripe webhook, or billing OFF) go in
#    deploy/.env.prod, which overlays deploy/.env key-by-key.
#      - deploy/.env.example lists the recognised keys.
#      - To turn billing OFF for prod, put empty lines in deploy/.env.prod:
#          STRIPE_SECRET_KEY=
#          STRIPE_WEBHOOK_SECRET=
#        (presence wins over value — an empty line means "this env has none").

# 3. (optional) Sanity-check which keys will be sealed — names only, no values:
deploy/scripts/seal-secrets.sh prod --show-keys

# 4. Seal. Uses the committed cert; reuses the cached stable values; writes the
#    encrypted file. No cluster contact.
deploy/scripts/seal-secrets.sh prod
#    → deploy/helm/beacon/sealed/prod/beacon-secrets.sealed.yaml

# 5. Verify (see Verification section), then commit + push (see What to commit).
```

`values-prod.yaml` already says `secretsSource: sealed`, so **no values-file edit is
needed** for the four base envs — the chart picks up the regenerated file
automatically. CI deploys on push (run the "Deploy to K3S" workflow), or add
`--apply` in step 4 if you have cluster access and want to apply immediately.

### Case B — brand-new env

The env has never been sealed and has no running Postgres to protect.

```bash
cd <repo-root>

# 1. Provide inputs in deploy/.env (+ deploy/.env.<env> if it needs overrides).

# 2. Seal — generates fresh stable values, writes the cache, encrypts:
deploy/scripts/seal-secrets.sh <env>
#    → deploy/helm/beacon/sealed/<env>/beacon-secrets.sealed.yaml
#    → deploy/.secrets/<env>.env   (NEW — gitignored, 0600)

# 3. BACK UP deploy/.secrets/<env>.env to your secret manager NOW.
#    It is the ONLY copy of the plaintext; the sealed file cannot be decrypted
#    locally, and POSTGRES_PASSWORD/ENCRYPTION_KEY must survive for the env's life.

# 4. If this is a NEW env (not one of int/acc/test/prod), also create its values
#    file `deploy/helm/beacon/values-<env>.yaml` with at least:
#        env: <env>
#        host: <public-hostname>
#        secretsSource: sealed
#    (copy an existing values-<env>.yaml as a template and change env/host).

# 5. Verify, commit + push.
```

---

## Verification (run before every commit)

```bash
# a) The sealed file is encrypted (must contain encryptedData, must NOT contain any
#    obvious plaintext value). Prints the key names only:
grep -E '^\s+[A-Z_]+:' deploy/helm/beacon/sealed/<env>/beacon-secrets.sealed.yaml | sed 's/:.*/: <encrypted>/'
grep -q encryptedData deploy/helm/beacon/sealed/<env>/beacon-secrets.sealed.yaml && echo "OK: encrypted"

# b) NOTHING sensitive is staged. Everything staged must be on the allowlist below.
git status --short
```

---

## What to commit

**Allowlist — only these may be staged:**

- `deploy/helm/beacon/sealed/<env>/beacon-secrets.sealed.yaml`  (and
  `beacon-registry.sealed.yaml` if you sealed a pull secret)
- `deploy/helm/beacon/values-<env>.yaml`  (only if you created/edited it — Case B step 4)
- `deploy/sealed/sealing-cert.pem`  (only if you deliberately refreshed the cert)

**Never stage:** `deploy/.env`, `deploy/.env.*`, `deploy/.secrets/*`, or anything
else containing plaintext. `.gitignore` blocks them; do not force them in.

```bash
git add deploy/helm/beacon/sealed/<env>/beacon-secrets.sealed.yaml
# (+ values-<env>.yaml only if you changed it)
git status --short          # confirm ONLY allowlisted paths are staged
git commit -m "chore(secrets): reseal <env> beacon-secrets"
git push
```

Then trigger a deploy: run the **Deploy to K3S** GitHub Actions workflow for that
env/brand (or `helm upgrade` manually). The controller unseals the committed
ciphertext into the plain `beacon-secrets` Secret the pods read via `secretKeyRef`.

---

## Two secrets that do NOT flow through kubeseal

1. **Frontend Google client id** — `NEXT_PUBLIC_GOOGLE_CLIENT_ID` is baked into the
   browser bundle at **build time** from the GitHub Actions secret `GOOGLE_CLIENT_ID`
   (`.github/workflows/deploy-k3s.yml`). It's public by nature (it ships to every
   browser), so it isn't sealed. The *backend* copy, `BEACON_GOOGLE_CLIENT_ID`, **is**
   sealed (it validates ID-token audience). Both must carry the same client id.
2. **Registry pull credentials** — either sealed as `beacon-registry`, or (current
   setup) the pre-existing `nebulacr-login` secret is referenced via
   `pullSecretCreate: false`. CI pushes images using the `NEBULACR_*` GitHub secrets.

---

## White-label brands (e.g. sysops) — no sealing step

Brand namespaces (`sysops-int`, …) use `secretsSource: existing`. The deploy pipeline
**copies** `beacon-secrets` + `nebulacr-login` from the base env namespace into the
brand namespace before Helm runs (`.github/workflows/deploy-k3s.yml`), so there is no
per-brand seal. For a brand **production** tier you want *dedicated* secrets instead:
seal them (`deploy/scripts/seal-secrets.sh <brand>-prod`) and set
`secretsSource: sealed` in that brand's values file.

---

## Rotation (deliberate, rare)

```bash
deploy/scripts/seal-secrets.sh <env> --rotate
```

Generates NEW random values. Consequences you must handle:

- **Postgres**: the DB role/volume must be reset to the new password, or delete the
  PVC so it re-inits (destroys data).
- **Encryption key**: tenant secrets encrypted with the old key become unreadable.
- **JWTs**: all existing sessions/tokens are invalidated (users re-login) — usually
  fine.

Only rotate JWT/webhook casually. Treat encryption-key and Postgres rotation as a
planned migration.

---

## Prerequisite the cluster needs (once per cluster)

The sealed-secrets controller must be installed, or nothing can decrypt these files:

```bash
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm upgrade --install sealed-secrets sealed-secrets/sealed-secrets \
  -n kube-system --set fullnameOverride=sealed-secrets-controller
```

If the controller is ever reinstalled its keypair changes; refresh the committed
cert and regenerate **every** sealed file:

```bash
kubeseal --fetch-cert > deploy/sealed/sealing-cert.pem   # needs cluster access
# then re-run seal-secrets.sh for each env (caches keep the values stable)
```
