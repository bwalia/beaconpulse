#!/usr/bin/env bash
#
# Fail the build unless a local image is built for the architecture the cluster
# actually runs.
#
# This exists because the failure it catches is silent and expensive: the CI
# runner is an arm64 Mac while every k3s1 node is amd64. A plain `docker build`
# happily produces an arm64 image, the registry happily accepts it, helm happily
# rolls it out — and only then does the kubelet die with
#
#     exec /usr/local/bin/api: exec format error
#
# which surfaces ten minutes later as an opaque "Progress deadline exceeded".
# Checking the manifest at build time turns that into an immediate, obvious error.
#
# Usage:  assert-image-arch.sh <image-ref> [expected-arch]
#         EXPECTED_ARCH=arm64 assert-image-arch.sh myimage:tag
set -euo pipefail

IMAGE="${1:-}"
EXPECTED="${2:-${EXPECTED_ARCH:-amd64}}"

[[ -n "$IMAGE" ]] || {
  echo "usage: $(basename "$0") <image-ref> [expected-arch]" >&2
  exit 2
}
command -v docker >/dev/null || {
  echo "error: docker is required" >&2
  exit 2
}

ARCH="$(docker image inspect "$IMAGE" --format '{{.Architecture}}' 2>/dev/null || true)"
OS="$(docker image inspect "$IMAGE" --format '{{.Os}}' 2>/dev/null || true)"

if [[ -z "$ARCH" ]]; then
  echo "::error::${IMAGE} is not in the local docker image store — did the build use --load?" >&2
  exit 1
fi

if [[ "$ARCH" != "$EXPECTED" ]]; then
  echo "::error::${IMAGE} is ${OS}/${ARCH}, but the cluster needs ${EXPECTED}." >&2
  echo "  The build host and the cluster differ in architecture. Build with" >&2
  echo "  'docker buildx build --platform linux/${EXPECTED}' — a plain 'docker build'" >&2
  echo "  silently targets the builder and the kubelet will fail with 'exec format error'." >&2
  exit 1
fi

echo "  arch OK: ${IMAGE} is ${OS}/${ARCH}"
