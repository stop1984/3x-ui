#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "$SCRIPT_DIR/fork-env.sh"

slug="manual"
binary="$REPO_ROOT/build/x-ui-patched"
service_name="x-ui"
target_path="/usr/local/x-ui/x-ui"
panel_url="https://127.0.0.1:41091/elmprod/"
skip_build=0
skip_frontend=0

usage() {
  cat <<'EOF'
Usage: bash scripts/fork-rollout.sh [options]

Options:
  --slug <name>         rollout slug used in /root/backups
  --binary <path>       binary to deploy (default: build/x-ui-patched)
  --service <name>      systemd service name (default: x-ui)
  --target <path>       live binary path (default: /usr/local/x-ui/x-ui)
  --panel-url <url>     smoke-check URL
  --skip-build          deploy existing binary without rebuilding
  --skip-frontend       skip frontend build during build step
  --print-toolchain     print resolved toolchain and exit
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --slug)
      shift
      slug=$1
      ;;
    --binary)
      shift
      binary=$1
      ;;
    --service)
      shift
      service_name=$1
      ;;
    --target)
      shift
      target_path=$1
      ;;
    --panel-url)
      shift
      panel_url=$1
      ;;
    --skip-build)
      skip_build=1
      ;;
    --skip-frontend)
      skip_frontend=1
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

if (( ! skip_build )); then
  build_args=(--output "$binary")
  if (( skip_frontend )); then
    build_args+=(--skip-frontend)
  fi
  bash "$SCRIPT_DIR/fork-build.sh" "${build_args[@]}"
fi

if [[ ! -x "$binary" && ! -f "$binary" ]]; then
  echo "Binary not found: $binary" >&2
  exit 1
fi

timestamp=$(date -u +%Y%m%dT%H%M%SZ)
rollout_dir="/root/backups/${timestamp}_${slug}"
mkdir -p "$rollout_dir"

cp "$target_path" "$rollout_dir/x-ui.pre-deploy"
cp "$binary" "$rollout_dir/x-ui.new"

systemctl stop "$service_name"
cp "$binary" "$target_path"
sha256sum "$target_path" | tee "$rollout_dir/sha256.txt"
systemctl start "$service_name"
systemctl is-active "$service_name" > "$rollout_dir/service-status.txt"

panel_ok=0
for _attempt in $(seq 1 30); do
  if curl -k -sS -I "$panel_url" > "$rollout_dir/panel-head.txt"; then
    panel_ok=1
    break
  fi
  sleep 1
done

if (( panel_ok == 0 )); then
  echo "Panel smoke-check did not answer: $panel_url" >&2
  exit 1
fi

printf '%s\n' "$rollout_dir"
