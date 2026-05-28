#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_PATH="${ROOT_DIR}/config.yaml"

if [[ ! -f "${CONFIG_PATH}" ]]; then
  echo "config.yaml not found. Put config.yaml in the project root." >&2
  exit 1
fi

echo "[start] building frontend"
(cd "${ROOT_DIR}/frontend" && pnpm build)

echo "[start] starting backend"
echo "[start] config: ${CONFIG_PATH}"
echo "[start] static: ${ROOT_DIR}/frontend/dist"

cd "${ROOT_DIR}/backend"
exec go run ./cmd/server -config "${CONFIG_PATH}" -static "${ROOT_DIR}/frontend/dist"
