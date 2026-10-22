#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

export GOCACHE="${GOCACHE:-$ROOT/.sandbox/gocache}"
export GOPATH="${GOPATH:-$ROOT/.sandbox/gopath}"
mkdir -p "$GOCACHE" "$GOPATH"

DATA_DIR="$(mktemp -d /tmp/mufish-e2e.XXXXXX)"

# 确保 E2E 使用最新的前端构建产物。
(cd "$ROOT/web" && npm run build >/dev/null)
go build -trimpath -o "$DATA_DIR/mufish-e2e" .

"$DATA_DIR/mufish-e2e" -addr 127.0.0.1:18899 -data "$DATA_DIR/data" &
child=$!
cleanup() {
  kill "$child" 2>/dev/null || true
  wait "$child" 2>/dev/null || true
  rm -rf "$DATA_DIR"
}
trap cleanup EXIT INT TERM
wait "$child"
