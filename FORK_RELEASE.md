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

`make verify` runs the local equivalent of the CI checks. Use GitHub Actions
for the canonical result before publishing. On small Linux hosts, Oxlint's
JavaScript-plugin worker may require swap.

## Versioning

Fork releases use:

```text
v<upstream-major>.<upstream-minor>.<upstream-patch>-stop1984.<revision>
```

Example:

```text
v3.11.0-stop1984.3
```

The release workflow injects the tag into `internal/config.buildVersion`.
Stable binaries therefore report the complete fork version, while untagged
development builds keep upstream's `dev+<commit>` behavior.

## Publishing

1. Confirm the branch is based on the intended upstream tag.
2. Confirm `git status` contains no generated or accidental files.
3. Run the local checks before pushing. Complete the full verification gate
   in Actions on the release tag before publishing.
4. Push the reviewed branch to `stop1984/3x-ui`. Installer and updater entry
   points resolve through the repository's default branch (`HEAD`).
5. Create and push the signed or annotated fork tag.
   If the push did not start the release workflow, dispatch it with the tag as
   its ref so the binary receives the stable version and the job stages a draft.
6. Wait for CI, all eight build jobs and the stable draft job. Verify the packaged
   `amd64` binary reports the fork version. For a database upgrade, rehearse
   the migration and smoke test with an isolated SQLite snapshot.
7. Make the reviewed branch the default and publish the complete draft as the
   latest stable release. Verify the public updater, `/releases/latest`, and the
   download and checksum URLs resolve to the intended repository and tag.

The workflow packages the panel, Xray-core, geodata and MTProto sidecar on its
supported platforms. Starting with `v3.11.0`, TUIC runs inside the panel. The
packaged build also includes the service units and shell menu.

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
4. Review the upstream release notes and the fork changelog for schema or
   protocol changes.
5. Ensure the built-in subscription profile mode and restart-on-client-disable
   behavior match the operator's intent.

For an existing VPSNL installation, use the panel's **Stable** channel or run
`sudo x-ui update`. To pin the same upgrade to a specific release from a root
shell, download and run that tag's updater:

```bash
XUI_UPDATE_TAG=v3.11.0-stop1984.3 \
  bash <(curl -fsSL https://raw.githubusercontent.com/stop1984/3x-ui/v3.11.0-stop1984.3/update.sh)
```

After rollout, verify:

- `x-ui -v` reports the complete fork version;
- the panel and Xray service are active;
- existing TLS, REALITY, XHTTP, mKCP, subscription, and node workflows still
  load;
- no migration or restart loop appears in logs;
- the existing `/json_freedom/` Xray JSON link still imports into v2rayN, and
  direct XHTTP entries omit empty `host` fields;
- mKCP with the intended TLS and FinalMask settings connects from a real client.

Rollback means restoring the complete prior application tree and database
backup together. Do not restore only the old binary after a schema migration.
