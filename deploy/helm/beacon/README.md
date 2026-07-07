# Beacon Helm chart

Deploys the full Beacon stack to a k3s cluster, one release per environment
(`int` / `test` / `acc` / `prod`), each in its own namespace.

## What it deploys

| Component | Kind | Image | Notes |
|---|---|---|---|
| api | Deployment + Service | `spectoncr.workstation.co.uk/beacon/api` | REST API; auto-runs migrations on start |
| worker | Deployment | `spectoncr.workstation.co.uk/beacon/worker` | reconciles Prometheus/Blackbox config |
| frontend | Deployment + Service | `spectoncr.workstation.co.uk/beacon/frontend` | Next.js dashboard |
| prometheus | Deployment + Service | `prom/prometheus` | reads Beacon-generated scrape jobs/rules |
| blackbox | Deployment + Service | `prom/blackbox-exporter` | probes; seeded then worker-managed |
| alertmanager | Deployment + Service | `prom/alertmanager` | webhook → Beacon API |
| prom-label-proxy-prom / -am | Deployment + Service | `prometheuscommunity/prom-label-proxy` | per-tenant `org_id` enforcement |
| nginx | Deployment + Service | `nginx` | single gateway; auth_request tenancy |
| postgres | Deployment + Service + PVC | `postgres:16` | in-cluster data store |
| redis | Deployment + Service + PVC | `redis:7` | in-cluster queue/cache |
| Ingress | Ingress | — | `wslproxy` class + cert-manager TLS → nginx |

The single public entry point is the nginx gateway, exposed at
`https://<env>.beaconpulse.net` (prod: `https://beaconpulse.net`).

## Cluster prerequisites

- **k3s** with the `local-path` storage provisioner (default).
- **External Secrets Operator** with a `ClusterSecretStore` named `vault-backend`
  (see `externalSecret` in `values.yaml`).
- **cert-manager** with a `ClusterIssuer` named `main-issuer`.
- **wslproxy** ingress controller (the platform ingress class).
- **Stakater Reloader** (optional but recommended) — rolls pods when
  `beacon-secrets` changes so rotated Vault values reach running pods.
- **Single node (or co-located control plane).** api/worker *write* the
  Prometheus generated files + Blackbox config to RWO PVCs that prometheus and
  blackbox *read*. On single-node k3s these share fine (all pods use
  `fsGroup: 10001`, no ownership conflict). For multi-node, set
  `storage.className` to an RWX provisioner (NFS/Longhorn) **or** pin the control
  plane with `controlPlaneNodeSelector`.

## Secrets (Vault)

Populate `secret/beaconpulse/<env>/config` with these keys — the ExternalSecret
syncs them into the `beacon-secrets` Secret:

| Key | Purpose |
|---|---|
| `BEACON_JWT_ACCESS_SECRET` | ≥32 bytes; access-token signing |
| `BEACON_JWT_REFRESH_SECRET` | ≥32 bytes; refresh-token signing |
| `BEACON_ENCRYPTION_KEY` | exactly 32 bytes (hex or raw); notification-secret encryption |
| `BEACON_WEBHOOK_TOKEN` | shared secret Alertmanager presents to the API webhook |
| `POSTGRES_PASSWORD` | in-cluster Postgres password (also builds the API DSN) |
| `BEACON_AI_API_KEY` | *(optional)* signing secret for the Ollama `x-api-key` JWT |
| `REGISTRY_USERNAME` | SpectonCR registry user — builds the `beacon-registry` image pull secret |
| `REGISTRY_PASSWORD` | SpectonCR registry password/token |

`prod` runs with `BEACON_ENV=production`, which **refuses to start** with default
("change-me") secrets — provide real values.

For **local testing without Vault**, set `externalSecret.enabled=false` and pass
values with `--set secrets.jwtAccess=… secrets.postgresPassword=…` (a plain
Secret is templated instead).

## Deploy

Via the GitHub Actions workflow (`.github/workflows/deploy-k3s.yml`) — push to
`main` auto build+deploys `int`; `workflow_dispatch` promotes to any env. It
builds/pushes the three images to the SpectonCR registry
(`spectoncr.workstation.co.uk/beacon/{api,worker,frontend}`, tagged with the
short SHA) and runs:

```sh
helm upgrade --install beacon ./deploy/helm/beacon \
  -f ./deploy/helm/beacon/values-<env>.yaml \
  --set image.tag=<sha> \
  --namespace <env> --create-namespace --wait
```

Required GitHub Actions secrets: `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`
(SpectonCR login), `KUBE_CONFIG_DATA_K3S` (base64 kubeconfig), `SLACK_WEBHOOK`
(optional). The cluster also needs `REGISTRY_USERNAME`/`REGISTRY_PASSWORD` in
Vault (`secret/beaconpulse/<env>/config`) for the `beacon-registry` image pull
secret.

### Manual deploy

```sh
helm upgrade --install beacon ./deploy/helm/beacon \
  -f ./deploy/helm/beacon/values-int.yaml \
  --namespace int --create-namespace
```

## Environments

| Env | Namespace | Host | BEACON_ENV | AI |
|---|---|---|---|---|
| int | `int` | int.beaconpulse.net | staging | on |
| test | `test` | test.beaconpulse.net | staging | off |
| acc | `acc` | acc.beaconpulse.net | staging | on |
| prod | `prod` | beaconpulse.net | production | on |

Point a DNS record for each host at the wslproxy ingress; cert-manager issues the
TLS cert via `main-issuer`.
