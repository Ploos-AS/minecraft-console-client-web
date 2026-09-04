# Minecraft Console Client Web

A small self-hosted WebAdmin for [Minecraft Console Client](https://github.com/MCCTeam/Minecraft-Console-Client), maintained by Ploos AS.

This project does **not** embed or fork MCC. It connects to MCC's external `WebSocketBot.cs` interface and keeps the MCC WebSocket password server-side.

## Status

The current implementation provides:

- Go backend with embedded responsive WebAdmin
- one shared authenticated MCC WebSocket session
- normalized browser protocol with correlated command responses
- structured Minecraft chat for public, private and raw chat events
- live MCC status cards for health, food, level, XP, TPS and world time
- observed player join/leave activity and last disconnect state
- separate MCC command input plus collapsible raw activity/event console
- native WebAdmin username/password login with server-side sessions
- authenticated UI, `/api/status` and browser WebSocket
- unauthenticated `/api/healthz` for container health checks
- graceful SIGTERM/SIGINT shutdown
- Alpine 3.22 runtime image
- non-root UID/GID 1000
- Docker Compose and rootless Podman examples
- CI for formatting, vet, tests, Go builds, container build and authenticated smoke testing

`WebSocketBot.cs` is an external MCC script and must be loaded in the MCC instance.

## Architecture

```text
Browser
   |
   | authenticated HTTP/WebSocket
   v
minecraft-console-client-web
   |
   | authenticated WebSocket, internal network
   v
Minecraft Console Client + WebSocketBot.cs
   |
   v
Minecraft Java server
```

Keep the MCC WebSocket endpoint on a private container or host network. Expose only the WebAdmin through your reverse proxy.

## MCC setup

Configure and load upstream `config/ChatBots/WebSocketBot.cs` in MCC. Use a strong unique WebSocket password and pass the same value to this service as `MCC_WS_PASSWORD`.

M0.7 consumes the upstream WebSocketBot events `OnChatPublic`, `OnChatPrivate`, `OnChatRaw`, `OnHealthUpdate`, `OnSetExperience`, `OnServerTpsUpdate`, `OnTimeUpdate`, `OnPlayerJoin`, `OnPlayerLeave`, `OnDisconnect` and `OnGameJoined` to build the structured WebAdmin view. The raw normalized event stream remains available under **Raw activity** for debugging and unsupported events.

The observed-player count reflects join/leave events seen by the current WebAdmin process; it is not presented as an authoritative server player count.

## Run locally

```bash
export MCC_WS_URL='ws://mcc:8043/'
export MCC_WS_PASSWORD='replace-mcc-password'
export MCC_WEB_PASSWORD='replace-webadmin-password'
go run ./cmd/mcc-web
```

The WebAdmin username defaults to `admin`. Override it with `MCC_WEB_USERNAME`.

The UI listens on `http://127.0.0.1:8080` when using the Compose example, or `:8080` when running the binary directly.

## Docker Compose

```bash
export MCC_WS_PASSWORD='replace-mcc-password'
export MCC_WEB_PASSWORD='replace-webadmin-password'
docker compose up --build -d
```

`compose.yaml` intentionally contains only the web application. MCC remains a separate service/container and should share a private network with the web application.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `MCC_WEB_LISTEN` | `:8080` | HTTP listen address |
| `MCC_WEB_USERNAME` | `admin` | WebAdmin login username |
| `MCC_WEB_PASSWORD` | none | Required WebAdmin login password |
| `MCC_WS_URL` | `ws://mcc:8043/` | MCC WebSocketBot endpoint |
| `MCC_WS_PASSWORD` | none | Required WebSocketBot password |

The MCC password is never sent to the browser. The Go backend authenticates the upstream MCC WebSocket session before accepting browser commands.

## WebAdmin authentication

Successful logins receive a cryptographically random, server-side session token in an `HttpOnly`, `SameSite=Strict` cookie. Sessions expire after 12 hours and are invalidated when the WebAdmin process restarts.

`/api/status`, the WebAdmin UI and `/ws` require a valid session. `/api/healthz` remains intentionally public so Docker, Podman and orchestration health checks do not need application credentials.

When HTTPS is terminated at a reverse proxy, forward `X-Forwarded-Proto: https` so the application marks the session cookie `Secure`. Use HTTPS whenever the WebAdmin is reachable beyond localhost or a trusted private network.

The current authentication model is single-user, not RBAC.

## Security model

The application keeps MCC credentials server-side, checks browser WebSocket origins against the request host, uses authenticated server-side sessions, sets defensive browser headers and runs as UID/GID 1000 with Linux capabilities dropped in the supplied Compose example.

Use strong, unrelated values for `MCC_WEB_PASSWORD` and `MCC_WS_PASSWORD`. Keep the MCC WebSocket service private even when WebAdmin authentication is enabled.

## Development

```bash
gofmt -w ./cmd
go vet ./...
go test ./...
go build ./cmd/mcc-web
docker build -t minecraft-console-client-web:dev .
```

## Upstream

Minecraft Console Client is maintained by MCCTeam and is a separate upstream project. This repository is an independent web interface and packaging project maintained by Ploos AS.
