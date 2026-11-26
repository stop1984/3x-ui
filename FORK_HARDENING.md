# stop1984 3X-UI Fork

The current integration merges upstream `v3.11.0` into the `v3.8.5-stop1984.2`
fork. The previous fork was rebuilt from upstream `v3.8.5` and had already
retired the superseded backend patches from the `v3.3.0` series.

## Current Delta

The fork intentionally carries a small behavioral delta:

1. Direct XHTTP exports omit an empty `host` instead of serializing an empty
   override. This applies to frontend links, raw subscription links, VMess
   objects, and JSON subscriptions.
2. The inbound form allows TLS with mKCP for stream protocols that support TLS.
3. mKCP exposes Salamander as a modern UDP FinalMask option. Inbound operators
   choose a mask explicitly. Switching to mKCP leaves the mask list alone.
   Existing mKCP Legacy configurations remain editable. The UI marks modern,
   legacy, and advanced mask families explicitly.
4. Install, update, menu, and panel self-update operations are pinned to
   `stop1984/3x-ui`; a fork release cannot silently replace itself with an
   upstream build.
5. Tagged builds report the complete fork version, for example
   `3.11.0-stop1984.3`. The shared version comparator compares the upstream
   version first, then the fork revision.
6. The updater checks the offline migration before starting the panel and
   reports migration and start failures through the update status file.
7. The inbound editor follows the server's TCP FinalMask/REALITY validation.
   It prevents additions with REALITY, lets the operator see and remove any
   stored TCP masks, and keeps them when security changes in the same draft.

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

# Both subscription formats must carry a client configuration that connects.
go test ./internal/sub -run '^TestXHTTPRealitySubscriptionConnects$' -count=1

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

1. Create an integration branch for the stable upstream tag and retain the
   prior fork tag as a rollback reference.
2. Audit whether upstream now implements each item in **Current Delta**.
3. Drop superseded fork code rather than preserving patch history mechanically.
4. Port remaining behavior with focused tests.
5. Run the complete verification gate.
6. Update this document and `CHANGELOG_FORK.md` before tagging.
