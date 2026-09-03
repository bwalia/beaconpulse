# Deploying a brand to a domain

The product has **two independent dimensions**:

| Dimension | Where it lives | Set at | 
|---|---|---|
| **Domain** (`int.sysops247.com`) | the Helm chart (`host`) | deploy time |
| **Brand** (name, logo, **colour**) | the frontend **image** (`NEXT_PUBLIC_BRAND`) | **build** time |

The chart is fully domain-portable — `host` alone drives the Ingress, TLS cert, CORS,
dashboard/Stripe URLs and Alertmanager. The **backend (api/worker) is brand-neutral** and
shared; only the **frontend** image carries a brand, because `src/brand/<name>.ts` is
inlined into the bundle at build time.

## One-click: deploy SysOps 24/7 (green)

SysOps ships with a full set of environments, exactly like beaconpulse:

| Env | Host | Namespace |
|---|---|---|
| int  | `int.sysops247.com`  | `sysops-int`  |
| test | `test.sysops247.com` | `sysops-test` |
| acc  | `acc.sysops247.com`  | `sysops-acc`  |
| prod | `sysops247.com`      | `sysops-prod` |

Each has its own `values-sysops-<env>.yaml` and its own wslproxy vhost. To deploy any of
them, go to **Actions → "Beacon Build/Push … Deploy to K3S" → Run workflow** and set:

- **BRAND** = `sysops`
- **TARGET_ENV** = `int` (or `test` / `acc` / `prod`)
- **DEPLOYMENT_TYPE** = `build-and-deploy`

That single run does everything, no manual step:

1. **DNS** — upserts `int.sysops247.com → lon1.pop0.uk` in Cloudflare. The zone id is
   resolved from the host's apex (`sysops247.com`), so no new secret is needed.
2. **wslproxy** — registers + activates the `int.sysops247.com` vhost from the committed
   `.github/wslproxy/data/{servers,rules}/prod/` files (edge TLS via Let's Encrypt).
3. **Build** — builds the frontend with `NEXT_PUBLIC_BRAND=sysops` → `frontend:sysops-<sha>`
   (Beacon's `frontend:latest` is untouched); api/worker build once, shared.
4. **Deploy** — `helm upgrade --install sysops` into the **`sysops-int`** namespace with
   `host=int.sysops247.com` and the branded frontend, reusing the brand-neutral backend.
   Secrets are **copied from the `int` namespace** into `sysops-int` automatically
   (`secretsSource: existing`), so there is no per-brand sealing.

Result: `https://int.sysops247.com` serving the green SysOps 24/7 dashboard.

### One-time prerequisites (not per-deploy, and not code)

- `sysops247.com` must be a **Cloudflare zone** in the same account, and the
  `CLOUDFLARE_API_TOKEN` must have **Zone:Read + DNS:Edit** on it. (An account-scoped
  token already covers it; a token scoped only to beaconpulse.net needs broadening.)
- **Beacon-int must have been deployed at least once** (it always is — every push to main
  deploys it), so `int` holds the `beacon-secrets` + `nebulacr-login` the brand copies.

## Deploy the same brand to another domain

1. Copy `values-sysops-int.yaml`, set a new `host:` (and `env:` for its namespace label).
2. Add a `.github/wslproxy/data/servers/prod/host:<domain>.json` (+ a rule) like the
   SysOps one.
3. Run the workflow with `BRAND=sysops`. The chart never hardcodes a domain — `host` is
   the only knob.

## RedFox Signals (orange) — the third brand, on redfoxsignals.com

Built exactly by the recipe below. Environments:

| Env | Host | Namespace | Secrets |
|---|---|---|---|
| int  | `int.redfoxsignals.com` | `redfox-int`  | copied from `int` (`existing`) |
| prod | `redfoxsignals.com` (+ `www`) | `redfox-prod` | dedicated, from wslvault (`external`) |

Deploy with **BRAND** = `redfox`, **TARGET_ENV** = `int` or `prod`.

**Before the first `prod` deploy**, populate its dedicated secrets — ESO reads
`kv/beaconpulse/redfox-prod/config`, and an ExternalSecret that never resolves fails
*quietly* (helm goes green, pods come up with no `POSTGRES_PASSWORD`):

    deploy/scripts/vault-load-secrets.sh redfox-prod

Unlike SysOps, redfox-prod is **not** pinned to cloud003 — a second full production
stack on that single edge node risks exhausting it, so RedFox Signals takes beaconpulse.net's
path through the k3s1 traefik ingress instead. See values-redfox-prod.yaml.

## Add a FOURTH brand

1. `frontend/src/brand/<brand>.ts` (copy `redfox.ts`; change name, ramp, mark) and
   register it in `src/brand/index.ts`.
2. Add `BRAND: <brand>` to the workflow's choice list.
3. Add `values-<brand>-int.yaml` (copy the RedFox overlay; change `host`) and the
   wslproxy vhost JSON — a server file per host, plus a `<brand>-prod-default` rule for
   the apex; for `int`, add the host to the shared `aws-k3s1-ingress-klipper-80` rule's
   `servers` list instead.
4. Run the workflow with `BRAND=<brand>`.
5. For the iOS app, see `ios/README.md` → "Add a brand".

## Colour / theme

Edit the `primary` 50→900 ramp in `frontend/src/brand/<brand>.ts` — that one block
re-tints the entire product (buttons, links, focus rings, logo). SysOps is currently green
(`#10b981`).

## Production brands: give them their own secrets

`secretsSource: existing` (copying int's secrets) is right for a shared **int** tier — SysOps-int
runs on the same JWT/encryption/DB creds as Beacon-int. For a **production** brand, provision
dedicated secrets instead: `deploy/scripts/seal-secrets.sh <brand>-prod`, commit
`sealed/<brand>-prod/`, and set `secretsSource: sealed` in its values file.

## What still does NOT switch with the brand

- **The favicon** — `frontend/src/app/icon.svg` is a static file still showing Beacon's
  mark. Replace it per brand, or make it brand-driven.
- **Backend user-facing text** — email subjects, the Stripe line item, health strings
  still say "Beacon". Separate backend change.
