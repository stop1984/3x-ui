#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

expected_repo='stop1984/3x-ui'
critical_files=(install.sh update.sh x-ui.sh internal/web/service/panel/panel.go)

for file in "${critical_files[@]}"; do
  if grep -Eqi 'https://(api\.github\.com/repos/|raw\.githubusercontent\.com/|github\.com/)(mhsanaei)/3x-ui' "$file"; then
    printf 'upstream update source leaked into %s\n' "$file" >&2
    exit 1
  fi
done

grep -q "$expected_repo" install.sh
grep -q "$expected_repo" update.sh
grep -q "$expected_repo" x-ui.sh
grep -q "$expected_repo" internal/web/service/panel/panel.go
grep -q 'XUI_REPO_BRANCH:-HEAD' install.sh
grep -q 'XUI_REPO_BRANCH:-HEAD' update.sh
grep -q 'XUI_REPO_BRANCH:-HEAD' x-ui.sh
grep -q 'panelRepositoryBranch = "HEAD"' internal/web/service/panel/panel.go
grep -q 'internal/config.buildVersion' .github/workflows/release.yml
grep -q 'ghcr.io/stop1984/3x-ui' .github/workflows/docker.yml
if grep -Eqi 'hsanaeii/3x-ui|ghcr\.io/mhsanaei/3x-ui' .github/workflows/docker.yml; then
  printf 'upstream container target leaked into .github/workflows/docker.yml\n' >&2
  exit 1
fi

bash -n install.sh
bash -n update.sh
bash -n x-ui.sh

printf 'fork release source checks: OK\n'
