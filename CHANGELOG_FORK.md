# Fork Changelog

## v3.11.0-stop1984.4 - 2026-11-26

- Switching an inbound to mKCP no longer adds a legacy UDP mask. The editor
  offers an explicit mask choice, including Salamander and mKCP Legacy.
- The inbound form explains the TCP FinalMask/REALITY conflict before saving.
  Existing TCP masks remain visible and removable.
- Added a loopback test that serves raw and JSON VLESS/XHTTP/REALITY
  subscriptions through the panel and makes a real HTTPS request through each
  exported VLESS outbound. The test also loads the full JSON profile in Xray
  and verifies that its routing selects the VLESS outbound.

## v3.11.0-stop1984.3 - 2026-11-25

- Updated the frontend lockfile to `follow-redirects` `1.16.3` for
  GHSA-mrwp-f3q8-3g9p. This release includes the Go dependency fixes from `.2`.
- The npm audit still reports the moderate GHSA-g3hv-75wg-m249 in the Swagger
  dependency chain. The current ApiDOM dependency requires minimatch 10, while
  the patched minimatch is 11.0.1. The existing `high` audit threshold passes.

## v3.11.0-stop1984.2 - 2026-11-25

- Updated Go to `1.27.4` for GO-2026-6859, Brotli to `1.2.7` for GO-2026-6888,
  and Gorilla WebSocket to `1.5.4-0.20240701034025-d67f41855da4` for GO-2026-6278.
- A manual Actions run on a stable tag can prepare the same draft as a tag
  push. Builds started from a branch still use development version metadata.
- This revision supersedes the unpublished `v3.11.0-stop1984.1` candidate.

## v3.11.0-stop1984.1 - 2026-11-25

Base: upstream `v3.11.0` (`c621ba9`), including Xray-core `v26.10.21`.

- Merged upstream `v3.9.0` through `v3.11.0` and retained the fork's XHTTP,
  mKCP TLS, and Salamander behavior. The UDP mask picker also keeps upstream's
  Sudoku, client-side UDP Hop and server-side source-IP filter.
- The panel and shell updater still use `stop1984/3x-ui`. The upstream shared
  version comparator now understands fork revision numbers, including in node
  compatibility checks.
- Updates run one checked database migration while the panel is stopped. The
  updater reports a failed service start as an update failure.
- The release workflow builds all eight packages, verifies their checksums and
  stages them in a draft for review before making the update available.
- Upstream now runs TUIC in-process. The archives no longer ship `tuic-server`.

## v3.8.5-stop1984.2 - 2026-09-26

Base: upstream `v3.8.5` (`7ef22f94c950ff09f0870e2295fa65ad5968742c`).

### Fixed

- The updater now runs database migrations while the panel service is stopped,
  preventing the service startup migration and the explicit migration command
  from racing on the same schema change.
- Update-time settings checks no longer pass stray positional values to boolean
  CLI flags.

## v3.8.5-stop1984.1 - 2026-09-26

Base: upstream `v3.8.5` (`7ef22f94c950ff09f0870e2295fa65ad5968742c`).

### Added

- mKCP TLS selection for compatible stream protocols.
- Salamander as a modern mKCP UDP FinalMask option.
- Explicit modern, legacy, and advanced labels in the UDP mask selector.
- Fork-aware installer, updater, menu, panel update checker, and tagged build
  version stamping.
- A manual container workflow scoped to `ghcr.io/stop1984/3x-ui`.

### Fixed

- Empty direct-XHTTP hosts are omitted from stored wire payloads, exported
  links, VMess objects, raw subscriptions, and JSON subscriptions.
- Fork releases no longer update themselves from `MHSanaei/3x-ui`.
- Fork release revisions compare correctly and do not trigger an endless
  same-version update prompt.
- The fork container workflow no longer targets upstream Docker Hub or GHCR
  namespaces.

### Upstream Integration

- Rebased from a clean upstream `v3.8.5` tag.
- Kept upstream's normalized client model, transactional mutation paths,
  current FinalMask implementation, node synchronization, and migration suite.
- Deliberately dropped the old fork backend patch series because upstream now
  implements those behaviors more broadly in the current `internal/`
  architecture.

### Upgrade Notes

- The built-in subscription profile page is disabled by default upstream.
- Manual client disable/delete may restart Xray when the corresponding setting
  is enabled.
- Port collisions are rejected before restart.
- Production rollout remains pending until the complete verification and backup
  procedure in `FORK_RELEASE.md` is complete.
