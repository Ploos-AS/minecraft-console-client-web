# M1.2 — Session controls and operational UX

## Goal

Turn the read-mostly WebAdmin dashboard into a safer operational console for the shared Minecraft Console Client session without changing the browser-to-MCC trust boundary.

## Scope

- Add explicit session actions for reconnecting the MCC WebSocket bridge and refreshing authoritative session state.
- Keep one shared upstream MCC session; browser tabs must never create independent MCC sessions.
- Reuse the existing authenticated browser protocol and command correlation.
- Make disruptive actions deliberate in the UI and provide immediate feedback.
- Refresh reruns the existing read-only hydration procedures without reconnecting MCC.
- Reconnect requests a controlled reconnect of the shared MCC WebSocket connection, fails pending requests cleanly, and then allows normal authentication/hydration to resume.
- Expose action progress/result without flooding Raw activity or chat.
- Preserve M1.1 chat deduplication, bridge observability, authentication, inventory/player state, and raw event visibility.

## Protocol

Add a browser request type for session actions:

```json
{"type":"session-action","id":"<browser request id>","action":"reconnect"}
```

The server returns a correlated normalized response. Unsupported actions are rejected explicitly.

`refresh` is a browser-local action that reruns the established read-only hydration command set; it does not need a new upstream MCC procedure.

## Safety

- Session actions require the existing authenticated `/ws` endpoint.
- No Microsoft/Minecraft credentials are exposed to the browser.
- Reconnect affects the shared upstream MCC WebSocket bridge only; it does not restart the WebAdmin process or container.
- No new host ports or external MCC WebSocket exposure.

## Qualification

- Go formatting and vet pass.
- Go tests cover parsing/validation and reconnect semantics.
- Existing command/text/session behavior remains green.
- WebAdmin and qualifier builds pass.
- OCI/container/auth smoke gates remain green.
- Manual UI behavior: refresh rehydrates state; reconnect visibly transitions and recovers without requiring a browser reload.
