# M0.2 runtime qualification

M0.2 verifies the web project's MCC protocol assumptions against a real Minecraft Console Client instance running the upstream `WebSocketBot.cs` script.

## Scope

The automated qualifier is intentionally read-only with respect to the Minecraft world. It verifies:

1. TCP/WebSocket connection to the configured WebSocketBot endpoint.
2. `Authenticate` succeeds using the configured password.
3. MCC emits `OnWsCommandResponse` using the documented nested JSON-string `data` payload.
4. A harmless `GetItemTypeMappings` command succeeds.

The qualifier does not send chat, move the player, manipulate inventory, or execute server-side commands.

## Prepare MCC

Use the upstream `config/ChatBots/WebSocketBot.cs` script and load it from MCC with:

```text
/script ChatBots/WebSocketBot.cs
```

Configure the bot with a bind address reachable by the qualifier, port 8043 (or another chosen port), and a strong unique password.

Keep the WebSocketBot endpoint on a trusted/private network. It should not be exposed directly to the Internet.

## Run the automated qualifier

From this repository:

```bash
export MCC_WS_URL='ws://127.0.0.1:8043/'
export MCC_WS_PASSWORD='replace-me'
go run ./cmd/mcc-qualify
```

Expected output:

```text
PASS auth
PASS command-response
PASS runtime-qualification
```

A non-zero exit status means qualification failed. The failing stage is printed to stderr.

## Web bridge qualification

After the direct protocol probe passes, run the web application against the same MCC instance:

```bash
export MCC_WS_URL='ws://127.0.0.1:8043/'
export MCC_WS_PASSWORD='replace-me'
go run ./cmd/mcc-web
```

Open the web UI and confirm the state reaches `connected`. Then verify these runtime cases:

- stop or restart `WebSocketBot.cs`; the browser remains connected to the web service and MCC state leaves `connected`;
- start the WebSocketBot again; the bridge reconnects and returns to `connected` without reloading the page;
- run a harmless MCC internal command from the UI, for example `/help`, and confirm the corresponding response/event is displayed;
- if a normal chat event occurs naturally, confirm it is forwarded to the browser unchanged.

## Qualification record

Record the MCC version/commit, WebSocketBot script version/commit, operating system/container image, architecture, date, and PASS/FAIL result before calling M0.2 qualified.

Do not mark the milestone runtime-qualified solely from unit tests or the mock-free direct build/smoke CI. A real MCC WebSocketBot endpoint is required for the final runtime gate.
