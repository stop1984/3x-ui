# Fork Build, Release, and Rollout

## Toolchain

- Go: the exact minimum in `go.mod` (`GOTOOLCHAIN=auto` is supported).
- Node: the major in `.nvmrc`.
- npm: the version bundled with that Node release.
- Linux release builds require the cross-toolchains downloaded by the GitHub
  Actions workflow.

Do not build the frontend with an older system Node. Husky and Oxlint use the
active `PATH`, including when invoked by `git commit`.

## Local Verification

Install frontend dependencies reproducibly:

```bash
cd frontend
npm ci
cd ..
```

Run focused fork checks while iterating:

```bash
GOTOOLCHAIN=auto go test ./internal/config ./internal/sub ./internal/web/service/panel
cd frontend
npm run typecheck
npx vitest run \
  src/test/stream-wire-normalize.test.ts \
  src/test/inbound-link.test.ts \
  src/test/protocol-capabilities.test.ts \
  src/test/finalmask.test.ts \
  --maxWorkers=1
cd ..
bash scripts/fork-check.sh
```

Run `make verify` before tagging. GitHub Actions is the canonical clean-machine
result. On small Linux hosts, Oxlint's JavaScript-plugin worker may require
swap; this is an upstream Oxlint runtime issue rather than a lint result.

## Versioning

Fork releases use:

```text
v<upstream-major>.<upstream-minor>.<upstream-patch>-stop1984.<revision>
```

Example:

```text
v3.8.5-stop1984.1
```

The release workflow injects the tag into `internal/config.buildVersion`.
Stable binaries therefore report the complete fork version, while untagged
development builds keep upstream's `dev+<commit>` behavior.

## Publishing

1. Confirm the branch is based on the intended upstream tag.
2. Confirm `git status` contains no generated or accidental files.
3. Run the full verification gate.
4. Push the reviewed branch to `stop1984/3x-ui` and make it the repository's
   default branch. Installer development sources resolve through `HEAD`, not a
   hard-coded branch name.
5. Create and push the signed or annotated fork tag.
6. Wait for every release matrix job to finish.
7. Verify every archive has a matching `.sha256` asset.
8. Confirm the release is marked stable and the installer resolves it through
   the fork's `/releases/latest` endpoint.

The workflow packages the panel, Xray-core, geodata, MTProto sidecar, and TUIC
sidecar. A locally built bare `x-ui` binary is not a complete release archive.

Container publishing is intentionally separate and manual. The
`Build 3X-UI Fork Container` workflow publishes only to
`ghcr.io/stop1984/3x-ui`; stable binary tags do not attempt to use upstream's
Docker Hub or GHCR namespaces.

## Rollout

Before changing a live installation:

1. Back up `/etc/x-ui`, the current `/usr/local/x-ui` tree, service overrides,
   and reverse-proxy configuration.
2. Record the current binary version and SHA-256.
3. Verify the release archive checksum.
4. Check the upstream release's **Before you upgrade** section.
5. Ensure the built-in subscription profile mode and restart-on-client-disable
   behavior match the operator's intent.

Prefer the tagged fork installer/updater so bundled sidecars remain in sync:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/stop1984/3x-ui/HEAD/install.sh) \
  v3.8.5-stop1984.1
```

After rollout, verify:

- `x-ui -v` reports the complete fork version;
- the panel and Xray service are active;
- existing TLS, REALITY, XHTTP, mKCP, subscription, and node workflows still
  load;
- no migration or restart loop appears in logs;
- a JSON subscription omits empty direct-XHTTP `host` fields;
- mKCP with the intended TLS and FinalMask settings connects from a real client.

Rollback means restoring the complete prior application tree and database
backup together. Do not restore only the old binary after a schema migration.
