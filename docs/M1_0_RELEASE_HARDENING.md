# M1.0 release hardening

M1.0 prepares the repository for the first stable OCI release without adding new MCC mutation features.

Implemented:

- exact `0.1.0` image references in stable Compose and Quadlet examples
- OCI source/version/revision labels in the runtime image
- multi-platform GHCR publishing for linux/amd64 and linux/arm64
- `edge` publication from `main`
- release-branch staging publication from `release/**`
- semver, minor, major and `latest` aliases from stable tags
- BuildKit SBOM and provenance attestations
- automatic GitHub Release creation after a successful stable-tag image build
- packaging validation in normal CI
- explicit release qualification checklist and immutable-tag policy

The work branch gate for the final M1.0 head must pass format, vet, tests, both Go builds, packaging checks, Docker build and authenticated container smoke test before merge.
