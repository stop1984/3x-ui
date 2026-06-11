# 3x-ui Fork Hardening Notes

This document tracks the source-level hardening work applied to the local
`3x-ui` fork instead of continuing to patch the panel via direct SQLite edits.
It started as a `v3.1.0` hardening branch and was later manually rebased onto
upstream `v3.3.0`.

The goal is straightforward:

- reduce `DB -> runtime config -> subscription -> UI` drift,
- eliminate destructive partial-update paths,
- keep behavior compatible with the active upstream branch without losing the
  local hardening guarantees,
- deploy only changes that were built, tested, and verified live.

## Scope

- Current upstream base: `3x-ui v3.3.0`
- Local repo: `/root/src/3x-ui-fork`
- Current working branch: `fork/v3.3.0-hardening`

## Rebase Status

The original hardening work was done on top of `v3.1.0`. During the manual
rebase to `v3.3.0`, some earlier fixes fell into three buckets:

1. kept as-is because upstream still had the same bug class,
2. adapted because the frontend/backend architecture changed,
3. dropped as obsolete where upstream had already replaced the old code path.

### Rebase-only adaptation commits

These commits are not part of the original `v3.1.0` hardening pass. They were
added while integrating the fork onto `v3.3.0`.

- `daf46bbe` - fix `DelInboundClient(..., forceRemove bool)` signature drift in
  rollback attach path after upstream changed the service API.
- `ecacc8f2` - adapt the subscription membership regression to the new
  `SubService.GetSubs(...)` return signature in `v3.3.0`.
- `2e3f3e5c` - fix frontend typing after the rebased client-save path landed in
  the newer React/Query stack.
- `08163eb1` - resolve traffic-owner lookups through canonical attached
  inbounds for both traffic-id and email lookups, instead of trusting stale
  `client_traffic.inbound_id`.
- `728f89a2` - sync generated OpenAPI docs so the repo reflects the new
  hardened client endpoints.

## Principles

1. Do not patch production state first and explain later.
2. Prefer source fixes over raw DB surgery.
3. When a bug is backend-shaped, fix it in the backend even if a frontend
   workaround exists.
4. Keep rollback simple:
   - backup current binary,
   - backup `x-ui.db`,
   - deploy one built artifact,
   - verify service health immediately.
5. Treat `3x-ui` as a multi-source system:
   - SQLite,
   - generated `config.json`,
   - subscription output,
   - frontend model state.

## Deployed Patch Series

### `640298cf` - Allow TLS on mKCP transports in form capabilities

Problem:

- the panel runtime already supported `mKCP + TLS`, and live inbounds could be
  configured that way,
- but the form capability matrix still treated `kcp` as TLS-ineligible, so the
  UI would not expose TLS for `mKCP` transports and users had to fall back to
  direct DB edits.

What changed:

- added `kcp` to the shared TLS-capability matrix used by both inbound and
  outbound forms,
- added explicit regression coverage for `vless/vmess/trojan + kcp + tls`,
- updated capability snapshots to lock the new behavior.

Effect:

- `mKCP + TLS` can now be configured directly in the panel UI,
- the form behavior matches the already-supported Xray runtime behavior.

### `1b9deb49` - Count tree depleted clients by canonical membership

Problem:

- `NodeService.GetNodeTree()` finishes with `recountByGuid()`, which rebuilds
  per-guid `InboundCount`, `OnlineCount`, and `DepletedCount` for the read-only
  node tree,
- that recount path still derived depletion from
  `client_traffics.inbound_id IN (node-owned inbounds)`,
- so a shared client attached to a node inbound and to a sibling local inbound
  could be counted correctly in `GetAll()` but then disappear again from the
  tree view if its shared traffic row still pointed at the local inbound.

What changed:

- changed the tree recount path to resolve membership through
  `clients + client_inbounds + inbounds`,
- built per-guid email sets from canonical attachments,
- loaded shared traffic truth by `email`,
- computed depleted counts from email-keyed traffic state instead of stale
  owner inbound ids,
- added regression coverage for a shared client attached to both a local and a
  node-owned inbound while the shared traffic row still points at the local
  inbound.

Effect:

- `GetNodeTree()` now agrees with `GetAll()` on depleted node clients,
- shared node clients no longer disappear from the tree view just because the
  shared traffic row was historically created under a local inbound.

### `3319a5c3` - Use canonical client state in traffic reset paths

Problem:

- reset paths still treated `client_traffics.enable` as if it were the whole
  source of truth,
- `disableInvalidClients()` already disables the shared client canonically:
  central `clients.enable`, embedded inbound settings, runtime user state, and
  `client_traffics.enable`,
- but `ResetClientTraffic*` and `BulkResetTraffic` only flipped
  `client_traffics.enable=true` and zeroed counters,
- that left an inconsistent state where traffic looked re-enabled while the
  client could still remain disabled in attached inbound settings and runtime.

What changed:

- changed `resetClientTrafficLocked()` so:
  - when the attached client is really disabled in settings, reset first
    re-enables the shared client through the canonical
    `SetClientEnableByEmail()` path,
  - when settings already say enabled but `client_traffics.enable=false`,
    reset keeps the old protective runtime-add path for that partial-drift
    scenario,
- changed `BulkResetTraffic()` to stop doing raw table updates and instead call
  the same canonical reset path per email,
- added regressions for:
  - shared-client reset re-enabling the central record plus every attachment,
  - bulk reset no longer leaving the embedded client disabled while only the
    traffic row changed.

Effect:

- reset and bulk-reset now agree with the fork's shared-client model,
- “traffic re-enabled, but client still disabled in settings/runtime” is no
  longer a normal outcome of a reset.

### `0fe91a16` - Return restart state from bulk traffic reset

Problem:

- after switching `BulkResetTraffic()` to the canonical per-email reset path,
  the controller still unconditionally called `SetToNeedRestart()`,
- that meant the UI could flag a restart even when the underlying reset path
  completed through live runtime updates and did not actually need one.

What changed:

- changed `BulkResetTraffic()` to return `needRestart` alongside `affected`,
- threaded that through the client controller,
- updated tests and service benchmarks for the new signature.

Effect:

- bulk reset now reports restart needs honestly instead of always forcing the
  panel into a pending-restart state.

### `5dd3fbb9` - Use canonical path for global traffic reset

Problem:

- the clients-page “reset all traffics” path still used a shallow global
  `UPDATE client_traffics SET up=0, down=0`,
- unlike inbound-scoped and bulk reset paths, it did not go through canonical
  shared-client re-enable logic,
- so a global reset could still leave clients disabled in the central record or
  in embedded inbound settings while the traffic rows looked reset.

What changed:

- changed `ClientService.ResetAllTraffics()` to enumerate traffic emails and
  route through the same canonical bulk reset path,
- threaded the new `inboundSvc` dependency through the controller and tests,
- added regression coverage that global reset now re-enables the shared client
  canonically instead of only zeroing counters.

Effect:

- global reset now matches the fork's shared-client model,
- clients-page “reset all traffics” no longer has weaker semantics than bulk
  reset or inbound-scoped reset.

### `301d1970` - Harden inbound flow normalization and update sync

Problem:

- inbound edits could leave invalid `flow` / `testseed` tails behind,
- `UpdateInbound` could operate on the wrong old snapshot,
- runtime/subscription/UI could disagree on whether `flow` was valid.

What changed:

- fixed old-snapshot handling in inbound updates,
- normalized `flow` and `testseed`,
- prevented runtime builder from emitting invalid `flow`,
- prevented subscription generation from leaking invalid `Trojan flow`,
- cleared invalid `VLESS flow` in the frontend form when transport/security no
  longer supports it.

Effect:

- fewer broken `REALITY -> TLS` / transport-transition leftovers,
- less silent drift between panel DB and live runtime.

### `356ba2ab` - Refresh inbound options and preserve raw inbound JSON

Problem:

- clients page kept stale inbound option metadata after inbound changes,
- inbound edit form eagerly normalized records through model conversion,
  dropping advanced or unknown fields before save.

What changed:

- refreshed inbound options on invalidation instead of only on first mount,
- initialized advanced stream/sniffing JSON from the raw DB payload,
- preserved unknown top-level keys while syncing advanced JSON back to the form.

Effect:

- less stale UI state,
- fewer advanced transport fields silently disappearing after edit.

### `3521a452` - Hydrate clients before toggling enable state

Problem:

- clients list intentionally uses a slim row payload,
- toggling `enable` from that row could send an incomplete client object to
  `/clients/update`,
- missing fields like `uuid`, `password`, `auth`, `flow`, `reverse` could be
  treated as replacement data.

What changed:

- before frontend toggle, the client is hydrated from `/clients/get/:email`,
- toggle path uses the full client record instead of the slim list row.

Effect:

- closed one destructive partial-update path immediately,
- bought safety before the backend-safe route existed.

### `5412b649` - Preserve unknown inbound settings keys on edit

Problem:

- even after preserving advanced `stream/sniffing`, ordinary inbound edits could
  still collapse unknown top-level `settings` keys back to model-known fields.

What changed:

- preserved unknown top-level `settings` keys during inbound form round-trip,
- merged only the protocol-managed keys instead of replacing the entire object.

Effect:

- better survival of non-standard settings,
- lower risk when experimenting with transports the panel UI only partially
  understands.

### `0519fbfe` - Fix multi-attach client mutation paths

Problem:

- several client mutations assumed `email -> one inbound`,
- but the `v3` model already has `client_inbounds`,
- helpers like:
  - `SetClientEnableByEmail`,
  - `ToggleClientEnableByEmail`,
  - `SetClientTelegramUserID`,
  - `ResetClientIpLimitByEmail`,
  - `ResetClientExpiryTimeByEmail`,
  - `ResetClientTrafficLimitByEmail`
  were effectively mutating whichever inbound happened to be returned first.

What changed:

- introduced a shared mutation path:
  - load canonical `ClientRecord`,
  - mutate `model.Client`,
  - fan out through `Update()` across all attached inbounds,
- changed enable/disable logic to use canonical client-record state,
- added dedicated backend route:
  - `POST /panel/api/clients/setEnable/:email`
- switched frontend toggle to that narrow route,
- updated API docs,
- added regression tests for multi-attach mutation behavior.

Effect:

- single-field client mutations are now consistent for multi-attach clients,
- no more “first inbound wins” behavior for these operations,
- frontend no longer needs to ship full client payloads just to toggle `enable`.

### `2bf07589` - Use canonical sub membership for subscription generation

Problem:

- subscription generation selected candidate inbounds from canonical
  `clients` / `client_inbounds` membership,
- but the actual per-link builders still filtered on embedded
  `inbound.settings.clients[].subId`,
- if one attached inbound carried a stale or empty embedded `subId`, the
  client was still canonically attached but silently disappeared from:
  - URI subscriptions,
  - JSON subscriptions,
  - Clash subscriptions.
- the `CopyInboundClients` path had the same class of drift when it minted a
  missing `subId` and only wrote it back to one inbound’s embedded client JSON.

What changed:

- added a canonical membership resolver for `(inboundID, subId)` based on the
  `clients` and `client_inbounds` tables,
- switched subscription builders to rehydrate per-inbound client data by email
  from canonical membership instead of trusting stale embedded `subId` fields,
- changed `CopyInboundClients` subId write-back to fan out through the shared
  client mutation path across all attached inbounds,
- added a regression test for stale embedded `subId` membership drift.

Effect:

- subscriptions now follow the canonical attachment graph,
- attached clients no longer disappear from one sub flavour just because one
  inbound JSON blob lagged behind,
- copy/import flows keep shared `subId` state aligned across attached inbounds.

### `d374956d` - Use canonical membership for inbound-wide traffic reset

Problem:

- `ResetAllClientTraffics(inboundId)` still reset traffic rows by
  `client_traffics.inbound_id`,
- shared clients attached to inbound `B` but “owned” by stale traffic row
  inbound `A` could be skipped entirely when resetting all traffic for `B`.

What changed:

- changed inbound-scoped traffic reset to resolve attached client emails from
  canonical `clients/client_inbounds` membership,
- batched traffic resets by `email IN (...)` instead of trusting the stale
  traffic owner inbound id,
- kept global `ResetAllClientTraffics(-1)` behavior unchanged,
- added regression coverage for shared-client reset through a sibling inbound.

Effect:

- inbound-wide traffic reset now matches the actual attachment graph,
- shared clients no longer miss resets just because their traffic row was
  historically created under a different attached inbound.

### `5433a2b1` - Use email-keyed traffic state in runtime inbound projection

Problem:

- runtime inbound projection for API paths filtered disabled clients by
  `client_traffics.inbound_id = inbound.Id`,
- shared client traffic rows are email-keyed and can carry a sibling inbound id,
  so an inbound could still expose a disabled shared client just because the
  traffic row owner pointed somewhere else.

What changed:

- changed `buildRuntimeInboundForAPI` to resolve enable state by client email,
  not by traffic owner inbound id,
- added regression coverage for a disabled shared client attached to two
  inbounds while the shared traffic row still points at the sibling inbound.

Effect:

- runtime/API inbound projection now matches the shared traffic truth,
- disabled shared clients no longer leak back into sibling inbound views after
  stale owner drift.

### `ef78c54f` - Use local attachments for auto-renew selection

Problem:

- periodic auto-renew still selected candidates through
  `client_traffics.inbound_id NOT IN (node inbounds)`,
- shared clients whose email-keyed traffic row still pointed at a node inbound
  could miss renewal entirely even though they were attached to a local inbound.

What changed:

- changed auto-renew candidate selection to start from expired shared traffic
  rows and then resolve local inbound membership through
  `clients/client_inbounds`,
- filtered renewal work to emails that still have at least one local attached
  inbound,
- kept remote-only rows out of the renewal path without trusting stale traffic
  owner inbound ids,
- added regression coverage for a shared client with a node-owned traffic row
  and a local attached inbound.

Effect:

- auto-renew now follows canonical local membership,
- shared clients no longer miss renewal just because their traffic row was
  historically created under a node inbound.

### `c941edf0` - Count depleted node clients by canonical membership

Problem:

- node summary `DepletedCount` still scanned `client_traffics` by
  `inbound_id IN (node-owned inbounds)`,
- shared clients attached to a node but carrying a local-owner traffic row
  could be exhausted, expired, or disabled without contributing to the node's
  depleted count.

What changed:

- changed node depleted counting to resolve node membership through
  `clients/client_inbounds`,
- aggregated email-keyed traffic state separately and joined it back to node
  membership by email,
- added regression coverage for a shared client attached to both a local and a
  node-owned inbound while the shared traffic row still points at the local
  inbound.

Effect:

- node depleted counts now follow canonical membership instead of stale owner
  inbound ids,
- shared node clients no longer disappear from depleted counts just because the
  shared traffic row was historically created under a local inbound.

### `d80bd892` - Make client edit + attachment sync a single backend operation

Problem:

- the clients page edit modal previously did three separate calls:
  1. `/clients/update/:email`
  2. `/clients/:email/attach`
  3. `/clients/:email/detach`
- if attach or detach failed after the client body had already been updated,
  the panel was left in a partially-applied state,
- rollback was effectively manual and depended on the user noticing the drift.

What changed:

- added a dedicated backend endpoint:
  - `POST /panel/api/clients/save/:email`
- introduced a backend save path that:
  - updates the client body,
  - syncs the exact attached inbound set,
  - attempts compensating rollback to the previous client snapshot and previous
    attachment set if attachment sync fails mid-flight,
- switched the clients page edit modal to use that single save path instead of
  frontend-orchestrated `update + attach + detach`,
- added a regression test that forces attachment sync failure after a successful
  client update and verifies rollback restores:
  - client record fields,
  - attached inbound IDs,
  - per-inbound embedded client state.

Effect:

- client edits are now much less likely to leave half-applied attachment state,
- rollback responsibility moved into the backend where the old snapshot is
  available,
- the edit modal no longer has to guess safe ordering for multi-step mutation.

### `7dc4d4c4` - Make direct attach/detach rollback-safe

Problem:

- even after the new `/clients/save/:email` path, the standalone attach/detach
  operations still mutated one inbound at a time with no rollback,
- if one inbound succeeded and a later inbound failed, the panel could keep a
  half-applied attachment set,
- this affected both direct `/attach` / `/detach` API calls and any internal
  code path reusing those helpers.

What changed:

- added compensating rollback inside `ClientService.Attach`:
  - if a later inbound fails, already-attached inbounds are detached again in
    reverse order,
- added compensating rollback inside `ClientService.Detach`:
  - if a later inbound fails, already-detached inbounds are re-attached from
    the canonical client snapshot,
- added regression tests for:
  - attach success on first inbound + failure on later inbound,
  - detach success on first inbound + failure on later inbound due to corrupt
    embedded client state.

Effect:

- direct attach/detach flows now behave transaction-like at the service layer,
- fewer half-applied attachment sets survive a mid-flight error,
- the rollback safety now exists below the modal/orchestration layer instead of
  only above it.

### `d57fe3e5` - Harden multi-inbound create and copy paths

Problem:

- `ClientService.Create` fanned a new client out across many inbounds with no
  rollback, so a failure on the second or third inbound could leave:
  - one attached inbound already mutated,
  - a partially-created shared client record,
  - leftover attachment state that the caller did not ask for.
- `InboundService.CopyInboundClients` also had a hidden source-side effect:
  - if the source client had an empty `subId`, copy generated one and wrote it
    back to the source before the target add succeeded,
  - if target add then failed, the copy operation returned an error but the
    source had already been mutated.

What changed:

- added rollback handling to multi-inbound `Create`:
  - if a later inbound fails, already-created attachments are detached again,
  - orphaned shared client records and leftover traffic/IP state are cleaned up,
- made copy generation side-effect free on the source side:
  - missing `subId` values are generated for the target copy payload only,
  - source client state is no longer written back as part of copy preparation,
- added regression tests for:
  - create success on the first inbound + failure on a later inbound,
  - copy failure on target add while the source client has an empty `subId`.

Effect:

- multi-inbound create now behaves transaction-like at the service layer,
- failed copy operations no longer leave hidden source `subId` mutations behind,
- fewer partially-created shared-client rows survive a failed create/copy path.

## Tests and Verification

Every deployed pass was gated by build/test verification.

Typical checks:

```bash
PATH=/root/toolchains/node-v22.22.3-linux-x64/bin:$PATH npm run typecheck
PATH=/root/toolchains/node-v22.22.3-linux-x64/bin:$PATH npm run build
PATH=/root/toolchains/go1.26.4/bin:$PATH go test ./...
PATH=/root/toolchains/go1.26.4/bin:$PATH go build -o build/x-ui-patched ./main.go
```

Additional targeted regression checks were added where the change deserved it,
for example multi-attach client mutation tests under:

- `web/service/client_multiattach_test.go`

And stale embedded sub membership coverage under:

- `sub/sub_service_membership_test.go`

And edit/attachment rollback coverage under:

- `web/service/client_multiattach_test.go`

And direct attach/detach rollback coverage under:

- `web/service/client_multiattach_test.go`

And multi-inbound create/copy rollback coverage under:

- `web/service/client_multiattach_test.go`
- `web/service/inbound_copy_test.go`

And delete rollback coverage under:

- `web/service/client_multiattach_test.go`

And reset-traffic shared-client coverage under:

- `web/service/client_multiattach_test.go`

And reset prevalidation coverage under:

- `web/service/client_multiattach_test.go`

And canonical shared-client email lookup coverage under:

- `web/service/client_multiattach_test.go`

Live deployment verification:

- replace `/usr/local/x-ui/x-ui` with the built binary,
- restart `x-ui.service`,
- confirm service is `active`,
- verify panel login page responds on:
  - `https://127.0.0.1:41091/elmprod/`

## Current Live State

At the time of writing, the local host has already deployed the patched binary
through commit:

- `e79342a1`

Local live binary:

- `/usr/local/x-ui/x-ui`

Rollback artifact for the latest rollout:

- `/root/backups/20260611T211313Z_xui_v330_search_lookup_canonical`

If this file is later pushed to GitHub, this section can be kept or trimmed;
the commit history above is the important public part.

## What This Work Has Improved

- less drift between SQLite and generated runtime config,
- safer transport/security transitions,
- fewer stale frontend snapshots,
- fewer destructive “replace-the-whole-client” side effects,
- correct multi-attach behavior for the most common client mutations,
- rollback-safe delete behavior when a later attached inbound fails mid-delete,
- tombstones only after successful delete instead of on failed partial delete,
- reset-traffic now uses the explicitly requested inbound instead of whichever
  stale inbound happened to own the shared `ClientTraffic.InboundId` row,
- multi-attach traffic reset now prevalidates every attached inbound before the
  first reset, so a corrupt later attachment cannot partially reset traffic,
- shared-client email lookup now resolves through canonical attachments instead
  of trusting the stale owner stored in `ClientTraffic.InboundId`,
- inbound-wide traffic reset now uses canonical attachment membership instead
  of the stale owner stored in `client_traffics.inbound_id`,
- runtime inbound projection now filters shared disabled clients by email-keyed
  traffic state instead of trusting `client_traffics.inbound_id`,
- periodic auto-renew now selects clients through local attachment membership
  instead of skipping shared rows whose stale owner points at a node inbound,
- node depleted counts now resolve client membership canonically instead of
  trusting `client_traffics.inbound_id` for node attribution,
- node tree depleted counts now use that same canonical membership model
  instead of regressing to stale traffic-owner inbound ids during the final
  per-guid recount step,
- traffic reset and bulk reset now re-enable the canonical shared client state
  instead of only flipping `client_traffics.enable`,
- bulk reset no longer marks the panel as needing restart unless one of the
  underlying per-email resets actually needed it,
- global traffic reset on the clients page now uses that same canonical reset
  path instead of directly rewriting `client_traffics`,
- tg-bot traffic lookup now resolves recipients from the canonical `clients`
  table (`tg_id`) instead of scanning embedded inbound settings, so shared and
  detached client records remain discoverable,
- search-traffic lookup now resolves client identity through the canonical
  `clients` table before falling back to legacy inbound JSON scans, and it
  rewrites the returned `InboundId` to the currently attached inbound instead
  of trusting a stale traffic-owner inbound id,
- `mKCP + TLS` can now be configured directly from the panel instead of only
  through manual DB/runtime edits,
- mKCP inbound FinalMask editing now exposes modern upstream UDP mask types
  like `salamander`, `mkcp-original`, `mkcp-aes128gcm`, `header-*`, and
  `sudoku` instead of forcing the old `mkcp-legacy`-only UI path,
- subscription/link-side finalmask normalization now preserves those modern
  UDP mask types instead of dropping them from `fm=` output.

## Known Limits / Not Yet Closed

These areas still deserve audit:

1. remaining reset flows where runtime-side effects can still drift from DB state,
2. remaining round-trip mismatches for exotic transport fields,
3. any remaining path that still trusts stale traffic ownership more than
   canonical `clients/client_inbounds` membership.

## Recommended Next Steps

1. Audit remaining reset flows where one runtime-side effect can succeed before
   a later attachment fails.
2. Compare `DB -> config.json -> links/sub` after inbound edits and fix the
   next concrete drift, not broad abstractions.
3. Continue replacing stale traffic-owner lookups with canonical attachment
   resolution where the service layer still mixes those concepts.
