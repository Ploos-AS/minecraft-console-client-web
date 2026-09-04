# M1.3 — WebAdmin correctness and polish

## Goal

Polish the WebAdmin behavior introduced in M1.1 and M1.2 before adding broader features. M1.3 is deliberately narrow: make chat deduplication conservative, make observability precise, and make session-action responses first-class browser protocol messages.

## Scope

- Treat `session-action-response` as a known browser message so successful reconnect requests never appear as `Unknown message` in Raw activity.
- Keep M1.2 reconnect status/results visible through the session controls UI.
- Make chat deduplication conservative and case-sensitive after Minecraft formatting/whitespace normalization; messages that differ only by case must remain distinct.
- Coalesce repeated activity errors only for MCC connection/bridge failures rather than unrelated application errors.
- Make the browser-to-WebAdmin bridge state explicitly distinguish `connecting`, `connected`, `retrying`, and `offline` in visible UI state.
- Preserve Raw activity as the complete normalized MCC event stream; chat deduplication remains presentation-only.
- Preserve the single shared MCC session, authentication, hydration, command correlation, inventory/player state, refresh, and controlled reconnect behavior.

## Non-goals

- No multi-instance support.
- No Microsoft/Minecraft credential management.
- No new host ports or direct browser exposure of the MCC WebSocketBot endpoint.
- No persistence/database work.
- No redesign of the MCC protocol bridge.

## Qualification

- Go formatting, vet, tests, WebAdmin/qualifier builds, OCI build, and auth/container smoke remain green.
- Frontend regression checks cover case-sensitive chat dedupe and known session-action responses where practical.
- Manual UI behavior: bridge state visibly transitions through connecting/connected/retrying/offline; reconnect response is not logged as unknown; unrelated identical errors are not coalesced.
