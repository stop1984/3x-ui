#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "$SCRIPT_DIR/fork-env.sh"

output="$REPO_ROOT/build/x-ui-patched"
run_frontend=1

usage() {
  cat <<'EOF'
Usage: bash scripts/fork-build.sh [--skip-frontend] [--output <path>] [--print-toolchain]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-frontend)
      run_frontend=0
      ;;
    --output)
      shift
      output=$1
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

mkdir -p "$(dirname -- "$output")"

if (( run_frontend )); then
  run_in_frontend "$NPM_BIN" run build
fi

run_in_repo "$GO_BIN" build -o "$output" ./main.go
sha256sum "$output"
