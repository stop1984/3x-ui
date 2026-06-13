# Fork Changelog

This file tracks fork-facing release notes separately from upstream changelogs.
It is intentionally short and focused on operator-visible differences.

## v3.3.0-stop1984.1

First public fork release, rebased on upstream `3x-ui v3.3.0`.

Highlights:

- rollback-safe client mutation paths for attach, detach, create, update, save, delete, and bulk operations,
- canonical client ownership through `clients + client_inbounds + client_traffics`,
- detached client preservation instead of destructive cleanup when inbounds disappear,
- safer reconciliation between DB state, generated runtime config, subscription output, and UI state,
- improved handling of legacy rows by bootstrapping them back into canonical state,
- panel support for modern `mKCP` / `FinalMask` workflows, including TLS on `mKCP`,
- release hygiene for the fork:
  - reproducible build/test helpers,
  - rollout helper scripts,
  - working multi-arch GitHub release assets.

Reference docs:

- [FORK_HARDENING.md](./FORK_HARDENING.md)
- [FORK_RELEASE.md](./FORK_RELEASE.md)
