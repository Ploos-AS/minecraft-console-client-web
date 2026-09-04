# M1.1 — Chat deduplication and bridge observability

M1.1 is the first post-v0.1.0 milestone.

## Goals

- Deduplicate chat lines when MCC emits both a structured chat event and the corresponding `OnChatRaw` event.
- Preserve raw-only chat lines; deduplication must never suppress unrelated messages.
- Keep the deduplication window bounded in time and memory.
- Surface browser-to-WebAdmin bridge state separately from MCC state.
- Make reconnect behavior visible without flooding the activity log with repeated identical state/errors.
- Preserve the existing shared upstream MCC session, authentication, hydration, command correlation, inventory, and read-only state behavior.

## Chat deduplication

The browser keeps a small rolling cache of recently rendered structured chat fingerprints. A subsequent raw chat event is suppressed only when its normalized text matches a recent structured event within a short window. Entries expire automatically and the cache has a hard size limit.

This is deliberately a presentation-layer dedupe. All normalized MCC events remain visible in Raw activity and no upstream events are discarded by the Go manager.

## Bridge observability

The dashboard bridge indicator distinguishes connecting, connected, retrying, and offline browser WebSocket states. Reconnect attempts use the existing browser retry loop, while repeated identical MCC connection errors are coalesced in the activity feed.

## Qualification

M1.1 must retain the normal repository gates: formatting, vet, Go tests, WebAdmin and qualifier builds, OCI build, and authenticated container smoke test.
