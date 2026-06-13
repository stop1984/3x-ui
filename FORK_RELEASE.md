# Fork Build, Test, and Rollout Guide

This fork already carries local hardening beyond upstream `v3.3.0`. The goal of
this document is narrower than [FORK_HARDENING.md](./FORK_HARDENING.md):

- make build and test steps reproducible,
- make live rollout predictable,
- make future rebases less dependent on ad hoc terminal history.

For public fork release notes, see [CHANGELOG_FORK.md](./CHANGELOG_FORK.md).

## Toolchain

Current validated toolchain:

- Go `1.26.4`
- Node `22.x`
- npm `10.x`

The helper scripts prefer explicitly configured binaries via environment
variables:

- `GO_BIN`
- `NODE_BIN`
- `NPM_BIN`

If those are not set, the scripts first try the local toolchain paths we have
used on this host:

- `/root/toolchains/go1.26.4/bin/go`
- `/root/toolchains/node-v22.22.3-linux-x64/bin/node`
- `/root/toolchains/node-v22.22.3-linux-x64/bin/npm`

If those paths do not exist, they fall back to whatever is on `PATH`.

## Helper Scripts

All helper scripts live under [`scripts/`](./scripts).

### 1. Test the fork

```bash
bash scripts/fork-test.sh
```

What it does:

- frontend `typecheck`
- frontend `build`
- backend `go test ./...`

Useful variants:

```bash
bash scripts/fork-test.sh --backend-only
bash scripts/fork-test.sh --frontend-only
```

### 2. Build the binary

```bash
bash scripts/fork-build.sh
```

Default output:

- `build/x-ui-patched`

Useful variants:

```bash
bash scripts/fork-build.sh --skip-frontend
bash scripts/fork-build.sh --output build/x-ui-custom
```

### 3. Roll out to the local host

```bash
bash scripts/fork-rollout.sh --slug my-change
```

Default behavior:

- builds `build/x-ui-patched`
- backs up the current live binary
- replaces `/usr/local/x-ui/x-ui`
- restarts `x-ui.service`
- waits for the panel HTTPS endpoint to answer
- writes rollout evidence into `/root/backups/<timestamp>_<slug>/`

Useful variants:

```bash
bash scripts/fork-rollout.sh --slug my-change --skip-build
bash scripts/fork-rollout.sh --slug my-change --skip-frontend
bash scripts/fork-rollout.sh --slug my-change --binary build/x-ui-patched
```

## Rollout Artifacts

Each rollout stores:

- `x-ui.pre-deploy`
- `x-ui.new`
- `sha256.txt`
- `service-status.txt`
- `panel-head.txt`

The rollout smoke-check intentionally treats any HTTP response from the panel as
"service answered". On this host the panel route often returns `404` for HEAD
requests even when the service is healthy, so the rollout check only treats
connection failure as fatal.

## Recommended Workflow

For code changes:

```bash
bash scripts/fork-test.sh
bash scripts/fork-rollout.sh --slug short-description
```

For docs-only or helper-script changes:

```bash
bash scripts/fork-test.sh --backend-only
```

## Notes for Future Rebases

When rebasing onto a newer upstream release:

1. rebase or cherry-pick onto a fresh branch from upstream,
2. run `bash scripts/fork-test.sh`,
3. compare against [FORK_HARDENING.md](./FORK_HARDENING.md),
4. only then roll forward to live.

The scripts do not replace judgment. They just make the routine parts less
fragile.
