# stop1984 3X-UI Fork

This branch is rebuilt from upstream `v3.8.5` rather than merging the old
`v3.3.0` fork history. The clean-base approach keeps upstream migrations,
security fixes, and the current normalized client model intact.

## Current Delta

The fork intentionally carries a small behavioral delta:

1. Direct XHTTP exports omit an empty `host` instead of serializing an empty
   override. This applies to frontend links, raw subscription links, VMess
   objects, and JSON subscriptions.
2. The inbound form allows TLS with mKCP for stream protocols that support TLS.
3. mKCP exposes Salamander as a modern UDP FinalMask option and uses it as the
   default for a newly added mKCP mask. The UI marks modern, legacy, and
   advanced mask families explicitly.
4. Install, update, menu, and panel self-update operations are pinned to
   `stop1984/3x-ui`; a fork release cannot silently replace itself with an
   upstream build.
5. Tagged builds report the complete fork version, for example
   `3.8.5-stop1984.1`, so the update checker does not offer the same release in
   a loop.

## Superseded Old Patches

The following old fork areas are not replayed because upstream `v3.8.5`
contains broader implementations and regression coverage:

- canonical client and inbound membership;
- multi-inbound create, edit, attach, detach, delete, and bulk operations;
- rollback and post-commit side-effect handling;
- detached-client and traffic-row preservation;
- node reconciliation and node-scoped client state;
- flow restoration and transport-aware VLESS flow gating;
- subscription generation from normalized tables;
- current FinalMask schemas, forms, migrations, and Xray-core validation.

Reintroducing the old service files would bypass the newer `internal/`
architecture and undo upstream fixes. Future rebases should preserve upstream's
implementation unless a current regression test demonstrates a missing case.

## Invariants

- No deployment procedure edits SQLite directly.
- Database migrations come from the current upstream codebase.
- An empty XHTTP host remains absent end to end.
- mKCP TLS affects form capability only; XTLS Vision flow remains restricted to
  valid protocol and transport combinations.
- FinalMask payloads use the current Xray-core schema. Legacy masks are retained
  only for compatibility and are labeled as such.
- Release archives and their SHA-256 sidecars are produced by the tagged commit.
- Container builds never publish to upstream Docker Hub or GHCR namespaces.
- The production binary is not replaced until tests, backup, and smoke checks
  have completed.

## Verification

The focused migration gate is:

```bash
GOTOOLCHAIN=auto go test ./internal/config ./internal/sub ./internal/web/service/panel

cd frontend
npm ci
npm run typecheck
npx vitest run \
  src/test/stream-wire-normalize.test.ts \
  src/test/inbound-link.test.ts \
  src/test/protocol-capabilities.test.ts \
  src/test/finalmask.test.ts \
  --maxWorkers=1
```

Before a release, run the full upstream gate:

```bash
make verify
bash scripts/fork-check.sh
```

The repository requires the Go version declared in `go.mod` and the Node major
declared in `.nvmrc`.

## Rebase Policy

For each upstream release:

1. Create a new branch from the stable upstream tag.
2. Audit whether upstream now implements each item in **Current Delta**.
3. Drop superseded fork code rather than preserving patch history mechanically.
4. Port remaining behavior with focused tests.
5. Run the complete verification gate.
6. Update this document and `CHANGELOG_FORK.md` before tagging.
