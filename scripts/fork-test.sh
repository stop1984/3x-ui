#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "$SCRIPT_DIR/fork-env.sh"

run_backend=1
run_frontend=1

usage() {
  cat <<'EOF'
Usage: bash scripts/fork-test.sh [--backend-only] [--frontend-only] [--print-toolchain]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --backend-only)
      run_frontend=0
      ;;
    --frontend-only)
      run_backend=0
      ;;
    --print-toolchain)
      print_toolchain_versions
      exit 0
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

if (( run_frontend )); then
  run_in_frontend "$NPM_BIN" run typecheck
  run_in_frontend "$NPM_BIN" run build
fi

if (( run_backend )); then
  run_in_repo "$GO_BIN" test ./...
fi
