# Release qualification

## Policy

Stable releases follow this path:

1. Merge a green `work/*` branch to `main`.
2. Confirm post-merge CI is green on the exact `main` commit.
3. Create `release/vX.Y.Z` from that exact commit.
4. Let the OCI workflow publish the release-branch staging image.
5. Qualify the staging image on both supported architectures where practical.
6. Create `vX.Y.Z` only from the qualified commit.
7. Confirm the tag workflow publishes exact semver, minor, major and `latest` aliases and creates the GitHub release.
8. Delete merged work/release branches after verification.

Do not move or overwrite an existing stable version tag.

## v0.1.0 gate

Before creating `v0.1.0`, verify:

- repository CI passes format, vet, unit tests, web build, qualifier build, Docker build and authenticated container smoke test
- staging image is an OCI multi-platform image with `linux/amd64` and `linux/arm64`
- container starts as UID/GID 1000
- `/api/healthz` succeeds without authentication
- `/`, `/api/status` and `/ws` remain protected by WebAdmin authentication
- successful login permits `/api/status`
- MCC WebSocket password is not exposed to the browser
- MCC reconnect restores authoritative state hydration
- read-only player/inventory views work with a real MCC WebSocketBot session
- SIGTERM produces a clean shutdown
- image carries OCI source/version/revision metadata
- SBOM and provenance attestations are present in the registry publication
- Compose and Quadlet examples reference `ghcr.io/ploos-as/minecraft-console-client-web:0.1.0`
- MCC WebSocket endpoint remains private; only WebAdmin is intended for reverse-proxy exposure

Record the exact release commit and resulting registry digest when qualification is performed. Do not claim independent registry verification until the manifest has actually been inspected.
