#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd -- "$SCRIPT_DIR/.." && pwd)

find_preferred_bin() {
  local fallback_name=$1
  shift
  local candidate
  for candidate in "$@"; do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  command -v "$fallback_name"
}

GO_BIN=${GO_BIN:-$(find_preferred_bin go /root/toolchains/go1.26.4/bin/go)}
NODE_BIN=${NODE_BIN:-$(find_preferred_bin node /root/toolchains/node-v22.22.3-linux-x64/bin/node)}
NPM_BIN=${NPM_BIN:-$(find_preferred_bin npm /root/toolchains/node-v22.22.3-linux-x64/bin/npm)}

prepend_dir_once() {
  local dir=$1
  case ":$PATH:" in
    *":$dir:"*) ;;
    *) PATH="$dir:$PATH" ;;
  esac
}

prepend_dir_once "$(dirname -- "$GO_BIN")"
prepend_dir_once "$(dirname -- "$NODE_BIN")"
prepend_dir_once "$(dirname -- "$NPM_BIN")"
export PATH

run_in_repo() {
  (
    cd "$REPO_ROOT"
    "$@"
  )
}

run_in_frontend() {
  (
    cd "$REPO_ROOT/frontend"
    "$@"
  )
}

print_toolchain_versions() {
  printf 'GO_BIN=%s\n' "$GO_BIN"
  "$GO_BIN" version
  printf 'NODE_BIN=%s\n' "$NODE_BIN"
  "$NODE_BIN" --version
  printf 'NPM_BIN=%s\n' "$NPM_BIN"
  "$NPM_BIN" --version
}
