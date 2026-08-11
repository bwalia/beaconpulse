#!/usr/bin/env bash
#
# seal-and-push.sh — one-command "edit .env → kubeseal → push a branch" wrapper.
#
# Interactive companion to seal-secrets.sh. It:
#   1. asks which environment (int|acc|test|prod),
#   2. asks for the env file that holds the input secrets (default: deploy/.env),
#   3. enforces the SECRETS.md safety rail (won't regenerate a LIVE env's stable
#      values without its plaintext cache — that would break Postgres),
#   4. runs deploy/scripts/seal-secrets.sh to produce the ENCRYPTED sealed file,
#   5. offers to commit ONLY that sealed file to a NEW branch and push it.
#
# You then raise the PR on GitHub; merging to main runs the auto-deploy.
#
# It NEVER commits deploy/.env or deploy/.secrets/* (plaintext). It commits with an
# explicit pathspec, so unrelated working-tree changes are never swept in.
#
# Usage:
#   deploy/scripts/seal-and-push.sh                 # fully interactive
#   deploy/scripts/seal-and-push.sh prod            # env preselected, rest prompted
#   deploy/scripts/seal-and-push.sh --help
#
# Requirements: kubeseal, kubectl, openssl, git (seal-secrets.sh checks the first
# three; this wrapper checks git). On this server they live in ~/bin.
set -euo pipefail

# Make ~/bin tools (kubeseal/kubectl installed there) visible to us and to the
# seal-secrets.sh child, even in a non-login shell.
case ":$PATH:" in *":$HOME/bin:"*) : ;; *) PATH="$HOME/bin:$PATH" ;; esac
export PATH

REPO_HOST="github.com"                       # for the printed PR URL only
# Accepted: bare tiers, or a white-label brand env (<brand>-<tier>, e.g. sysops-prod).
ENV_HINT="int|acc|test|prod  (or a brand env like sysops-prod)"
is_valid_env() {
  case "$1" in
    int|acc|test|prod|*-int|*-acc|*-test|*-prod) return 0 ;;
    *) return 1 ;;
  esac
}

c_red=$'\033[31m'; c_grn=$'\033[32m'; c_yel=$'\033[33m'; c_bld=$'\033[1m'; c_rst=$'\033[0m'
say()  { printf '%s\n' "$*"; }
info() { printf '%s\n' "  $*"; }
ok()   { printf '%s%s%s\n' "$c_grn" "  ✔ $*" "$c_rst"; }
warn() { printf '%s%s%s\n' "$c_yel" "  ! $*" "$c_rst"; }
die()  { printf '%s%s%s\n' "$c_red" "error: $*" "$c_rst" >&2; exit 1; }
rule() { printf '%s\n' "────────────────────────────────────────────────────────────"; }

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  sed -n '2,31p' "$0" | sed 's/^# \{0,1\}//'; exit 0
fi

# ── locate the repo (works no matter where you invoke it from) ───────────────
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" \
  || die "not inside a git repository. cd into the beaconpulse checkout first."
cd "$REPO_ROOT"
SEAL="$REPO_ROOT/deploy/scripts/seal-secrets.sh"
[ -x "$SEAL" ] || die "missing $SEAL — is this the beaconpulse repo?"

# ── preflight: the tools seal-secrets.sh needs ──────────────────────────────
missing=""
for t in git openssl kubectl kubeseal; do command -v "$t" >/dev/null 2>&1 || missing="$missing $t"; done
if [ -n "$missing" ]; then
  say "${c_red}Missing required tools:${c_rst}$missing"
  say ""
  say "Install them (arm64 macOS, user-space, no sudo):"
  say "  mkdir -p ~/bin"
  say "  # kubectl"
  say '  curl -L "https://dl.k8s.io/release/$(curl -Ls https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl" -o ~/bin/kubectl && chmod +x ~/bin/kubectl'
  say "  # kubeseal"
  say '  curl -L https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.27.1/kubeseal-0.27.1-darwin-arm64.tar.gz | tar -xz -C /tmp kubeseal && mv /tmp/kubeseal ~/bin/ && chmod +x ~/bin/kubeseal'
  say "  # then re-run this script"
  exit 1
fi

# ── 1. which environment ────────────────────────────────────────────────────
ENV_ARG="${1:-}"
if [ -n "$ENV_ARG" ]; then
  TARGET_ENV="$ENV_ARG"
else
  say "${c_bld}Which environment do you want to seal?${c_rst}  ($ENV_HINT)"
  printf "  env [int]: "; read -r TARGET_ENV || true
  [ -n "$TARGET_ENV" ] || TARGET_ENV="int"
fi
is_valid_env "$TARGET_ENV" || die "invalid env '$TARGET_ENV' (expected $ENV_HINT)"
# A brand env must have its values file, else the deploy has nothing to use.
if [ ! -f "$REPO_ROOT/deploy/helm/beacon/values-$TARGET_ENV.yaml" ]; then
  die "no values file: deploy/helm/beacon/values-$TARGET_ENV.yaml (create it before sealing '$TARGET_ENV')"
fi
ok "environment: $TARGET_ENV"

# ── 2. which env file holds the inputs (AI/Google/Stripe/registry) ──────────
DEFAULT_ENV_FILE="$REPO_ROOT/deploy/.env"
say ""
say "${c_bld}Path to the env file with your input secrets${c_rst}"
info "(AI key, Google client id, Stripe, registry creds — the values you edit)"
printf "  env file [%s]: " "deploy/.env"; read -r ENV_FILE_IN || true
if [ -z "$ENV_FILE_IN" ]; then
  ENV_FILE="$DEFAULT_ENV_FILE"
else
  case "$ENV_FILE_IN" in /*) ENV_FILE="$ENV_FILE_IN" ;; *) ENV_FILE="$REPO_ROOT/$ENV_FILE_IN" ;; esac
fi
[ -f "$ENV_FILE" ] || die "env file not found: $ENV_FILE
       Create it first (copy deploy/.env.example) and put your secrets in it."
ok "input file: ${ENV_FILE#"$REPO_ROOT"/}"

# ── 3. SAFETY RAIL: never regenerate a live env's stable values ─────────────
# POSTGRES_PASSWORD and BEACON_ENCRYPTION_KEY must stay stable for an env's life.
# seal-secrets.sh keeps them in deploy/.secrets/<env>.env. If a sealed file already
# exists (env is live) but that cache is absent on THIS machine, sealing would mint
# brand-new values and break the running database. Refuse — get the cache first.
SEALED_FILE="$REPO_ROOT/deploy/helm/beacon/sealed/$TARGET_ENV/beacon-secrets.sealed.yaml"
CACHE_FILE="$REPO_ROOT/deploy/.secrets/$TARGET_ENV.env"
say ""
if [ -f "$SEALED_FILE" ] && [ ! -f "$CACHE_FILE" ]; then
  rule
  say "${c_red}${c_bld}REFUSING: '$TARGET_ENV' is already live but its secret cache is missing here.${c_rst}"
  rule
  info "A sealed file exists:   ${SEALED_FILE#"$REPO_ROOT"/}"
  info "But the cache is absent: deploy/.secrets/$TARGET_ENV.env"
  say ""
  info "Sealing now would generate NEW jwt/encryption/postgres values and the running"
  info "Postgres would fail to authenticate against its existing volume."
  say ""
  say "  ${c_bld}Fix:${c_rst} copy the cache from the machine that last sealed $TARGET_ENV (securely):"
  info "    scp <that-host>:.../deploy/.secrets/$TARGET_ENV.env deploy/.secrets/"
  info "  then re-run this script."
  say ""
  info "Only if you INTEND a destructive rotation (and will reset the DB), run manually:"
  info "    deploy/scripts/seal-secrets.sh $TARGET_ENV --rotate"
  exit 1
elif [ ! -f "$SEALED_FILE" ] && [ ! -f "$CACHE_FILE" ]; then
  warn "brand-new env '$TARGET_ENV': fresh values will be generated and cached."
  warn "After sealing, BACK UP deploy/.secrets/$TARGET_ENV.env — it's the only plaintext copy."
else
  ok "cache present (stable values will be reused): deploy/.secrets/$TARGET_ENV.env"
fi

# ── point seal-secrets.sh at the chosen env file (only if it isn't deploy/.env) ─
# seal-secrets.sh reads $REPO_ROOT/deploy/.env. If you chose a different file we
# temporarily stand it in, and ALWAYS restore the original on exit (even on Ctrl-C),
# so your real deploy/.env is never left clobbered.
SWAP_ACTIVE=false; SWAP_BACKUP=""
restore_env_file() {
  [ "$SWAP_ACTIVE" = true ] || return 0
  if [ -n "$SWAP_BACKUP" ]; then cp -p "$SWAP_BACKUP" "$DEFAULT_ENV_FILE"; rm -f "$SWAP_BACKUP";
  else rm -f "$DEFAULT_ENV_FILE"; fi
  SWAP_ACTIVE=false
}
trap 'restore_env_file' EXIT INT TERM
if [ "$ENV_FILE" != "$DEFAULT_ENV_FILE" ]; then
  if [ -e "$DEFAULT_ENV_FILE" ]; then SWAP_BACKUP="$(mktemp)"; cp -p "$DEFAULT_ENV_FILE" "$SWAP_BACKUP"; fi
  cp -p "$ENV_FILE" "$DEFAULT_ENV_FILE"
  SWAP_ACTIVE=true
  info "using ${ENV_FILE#"$REPO_ROOT"/} as deploy/.env for this run (restored afterwards)"
fi

# ── 4. show which keys will be sealed (NAMES ONLY — no values) ──────────────
say ""
say "${c_bld}Keys that will be sealed for '$TARGET_ENV':${c_rst}"
"$SEAL" "$TARGET_ENV" --show-keys || die "seal-secrets.sh --show-keys failed"

say ""
printf "%sProceed to kubeseal '%s'? [y/N]: %s" "$c_bld" "$TARGET_ENV" "$c_rst"; read -r yn || true
case "$yn" in y|Y|yes|YES) : ;; *) die "aborted before sealing (nothing changed)";; esac

# ── 5. seal ─────────────────────────────────────────────────────────────────
say ""; rule; say "Sealing $TARGET_ENV ..."; rule
"$SEAL" "$TARGET_ENV"
restore_env_file   # put deploy/.env back before we touch git
trap - EXIT INT TERM
[ -f "$SEALED_FILE" ] || die "expected sealed file was not produced: $SEALED_FILE"
grep -q encryptedData "$SEALED_FILE" || die "sealed file has no encryptedData — refusing to commit"
ok "sealed (encrypted): ${SEALED_FILE#"$REPO_ROOT"/}"

# ── 6. commit ONLY the sealed dir to a NEW branch, then push ────────────────
SEALED_DIR="deploy/helm/beacon/sealed/$TARGET_ENV"
say ""
say "${c_bld}Changes to publish:${c_rst}"
git -c color.status=always status --short -- "$SEALED_DIR" || true
say ""
printf "%sPush these sealed secrets to a new branch on GitHub? [y/N]: %s" "$c_bld" "$c_rst"; read -r push || true
case "$push" in
  y|Y|yes|YES) : ;;
  *) say ""; info "Not pushed. The sealed file is on disk; commit it yourself when ready:";
     info "  git checkout -b secrets/reseal-$TARGET_ENV && git add $SEALED_DIR && git commit && git push -u origin HEAD";
     exit 0 ;;
esac

STAMP="$(date +%Y%m%d-%H%M%S)"
BRANCH="secrets/reseal-${TARGET_ENV}-${STAMP}"
git checkout -b "$BRANCH"

# Stage ONLY the sealed dir, then assert nothing outside it slipped into the index.
git add -- "$SEALED_DIR"
staged="$(git diff --cached --name-only)"
if [ -z "$staged" ]; then
  git checkout - >/dev/null 2>&1 || true
  die "nothing staged under $SEALED_DIR — did the seal actually change anything?"
fi
for f in $staged; do
  case "$f" in
    "$SEALED_DIR"/*) : ;;
    *) die "SAFETY STOP: '$f' is staged but is outside $SEALED_DIR. Refusing to commit.";;
  esac
done

git commit -m "chore(secrets): reseal ${TARGET_ENV} beacon-secrets" -- "$SEALED_DIR"
git push -u origin "$BRANCH"

# Derive owner/repo from the remote WITHOUT printing it (it may embed a token).
slug="$(git remote get-url origin | sed -E 's#^.*'"$REPO_HOST"'[:/]##; s#\.git$##')"
say ""; rule
ok "pushed branch: $BRANCH"
say "${c_bld}Open the PR:${c_rst}"
info "https://$REPO_HOST/$slug/compare/main...$BRANCH?expand=1"
say ""
info "Merging to main runs the auto-deploy (push→main builds & deploys to int)."
info "For test/acc/prod, run the 'Deploy to K3S' workflow with TARGET_ENV after merge."
rule
