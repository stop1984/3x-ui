# Fork Changelog

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
