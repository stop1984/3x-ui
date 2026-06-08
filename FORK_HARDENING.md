# 3x-ui Fork Hardening Notes

This document tracks the source-level hardening work applied to the local
`3x-ui` fork instead of continuing to patch the panel via direct SQLite edits.

The goal is straightforward:

- reduce `DB -> runtime config -> subscription -> UI` drift,
- eliminate destructive partial-update paths,
- keep behavior compatible with the upstream `v3.1.0` data model,
- deploy only changes that were built, tested, and verified live.

## Scope

- Upstream base: `3x-ui v3.1.0`
- Local repo: `/root/src/3x-ui-fork`
- Working branch: `fork/v3.1.0-hardening`

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

### `TBD` - Use canonical sub membership for subscription generation

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

Live deployment verification:

- replace `/usr/local/x-ui/x-ui` with the built binary,
- restart `x-ui.service`,
- confirm service is `active`,
- verify panel login page responds on:
  - `https://127.0.0.1:41091/elmprod/`

## Current Live State

At the time of writing, the local host has already deployed the patched binary
through commit:

- `0519fbfe`

Local live binary:

- `/usr/local/x-ui/x-ui`

Rollback artifact for the latest rollout:

- `/root/backups/20260608T214959Z_xui_fork_patch_deploy_5`

If this file is later pushed to GitHub, this section can be kept or trimmed;
the commit history above is the important public part.

## What This Work Has Improved

- less drift between SQLite and generated runtime config,
- safer transport/security transitions,
- fewer stale frontend snapshots,
- fewer destructive “replace-the-whole-client” side effects,
- correct multi-attach behavior for the most common client mutations.

## Known Limits / Not Yet Closed

These areas still deserve audit:

1. client attach/detach flows,
2. remaining round-trip mismatches for exotic transport fields,
3. any path that still treats the first matching inbound as authoritative for a
   shared client identity.

## Recommended Next Steps

1. Audit attach/detach flows for the same partial-update class.
2. Compare `DB -> config.json -> links/sub` after inbound edits and fix the
   next concrete drift, not broad abstractions.
3. Keep pushing backend-safe narrow endpoints where the UI currently relies on
   full-object updates for single-field mutations.
