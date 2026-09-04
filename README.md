# Minecraft Console Client Web

A small self-hosted WebAdmin for [Minecraft Console Client](https://github.com/MCCTeam/Minecraft-Console-Client), maintained by Ploos AS.

This project does **not** embed or fork MCC. It connects to MCC's external `WebSocketBot.cs` interface and keeps the MCC WebSocket password server-side.

## Status

Early bootstrap / M0. The current implementation provides:

- Go backend with embedded static frontend
- browser-to-MCC WebSocket bridge
- server-side MCC WebSocket authentication
- live display of MCC WebSocket events
- send chat text and MCC `/commands` from the browser
- `/api/healthz` and `/api/status`
- graceful SIGTERM/SIGINT shutdown
- Alpine 3.22 runtime image
- non-root UID/GID 1000
- Docker Compose example
- rootless Podman Quadlet example
- CI for formatting, vet, tests, Go build, container build and smoke test

The MCC WebSocket bridge is based on the upstream WebSocket Bot protocol documented by MCCTeam. `WebSocketBot.cs` is an external MCC script and must be loaded in the MCC instance.

## Architecture

```text
Browser
   |
   | HTTP/WebSocket
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

The MCC WebSocket endpoint should normally stay on a private container or host network. Only the web application should be exposed through your reverse proxy.

## MCC setup

Configure and load the upstream `config/ChatBots/WebSocketBot.cs` script in MCC. The upstream script accepts a bind address, port and password and is loaded with MCC's `/script` command.

Use a strong unique password. Pass the same password to this service as `MCC_WS_PASSWORD`.

## Run locally

```bash
export MCC_WS_URL='ws://mcc:8043/'
export MCC_WS_PASSWORD='replace-me'
go run ./cmd/mcc-web
```

The UI listens on `http://127.0.0.1:8080` when using the Compose example, or `:8080` when running the binary directly.

## Docker Compose

```bash
export MCC_WS_PASSWORD='replace-me'
docker compose up --build -d
```

`compose.yaml` intentionally contains only the web application. MCC remains a separate service/container and should share a private network with the web application.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `MCC_WEB_LISTEN` | `:8080` | HTTP listen address |
| `MCC_WS_URL` | `ws://mcc:8043/` | MCC WebSocketBot endpoint |
| `MCC_WS_PASSWORD` | none | Required WebSocketBot password |

The password is never sent to the browser. The Go backend authenticates the upstream MCC WebSocket session before bridging traffic.

## Security model

The current M0 UI has no end-user authentication of its own. Bind it to localhost or a trusted private network and put authentication at the reverse proxy until native auth lands in a later milestone.

The container runs as UID/GID 1000, drops Linux capabilities in the supplied Compose/Quadlet examples, and is designed to run without root privileges.

## Planned milestones

M1 will turn the raw event console into a real MCC WebAdmin: structured connection state, chat rendering, command responses, inventory/status views and safer reconnect handling. Later milestones can add multi-instance management, saved macros, native authentication/RBAC and richer MCC event views.

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
