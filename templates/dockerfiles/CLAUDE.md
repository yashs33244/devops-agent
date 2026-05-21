# templates/dockerfiles/

Multi-stage, distroless, non-root Dockerfile templates for five language runtimes. Rendered by `tools/dockerize.py`.

## What's Here

```
python/
  Dockerfile                    # python:3.12-slim-bookworm → distroless/python3-debian12:nonroot (~60 MB)
  container-structure-test.yaml # Container structure tests
go/
  Dockerfile                    # golang:1.23-alpine → distroless/static-debian12:nonroot (~5-15 MB)
  container-structure-test.yaml
node/
  Dockerfile                    # node:22-alpine → distroless/nodejs22-debian12:nonroot (~120 MB)
  container-structure-test.yaml
java/
  Dockerfile                    # eclipse-temurin:21-jdk-alpine → eclipse-temurin:21-jre-alpine (~220 MB)
  container-structure-test.yaml
rust/
  Dockerfile                    # rust:1.82-alpine → distroless/static-debian12:nonroot (~5-15 MB)
  container-structure-test.yaml
README.md                       # Full usage guide, digest pinning instructions, scanning commands
```

## Template Variables

Replace before building (or let `tools/dockerize.py` substitute automatically):

| Variable | Description | Example |
|----------|-------------|---------|
| `{{PORT}}` | App listen port | `8000`, `8080` |
| `{{BINARY_NAME}}` | Go/Rust output binary | `server` |
| `{{MAIN_MODULE}}` | Python uvicorn module path | `main:app` |

## How to Use / Run

```bash
# Always build targeting the runner stage
docker build --target runner -t myservice:1.0.0 python/

# Run container structure tests
container-structure-test test \
  --image myservice:1.0.0 \
  --config python/container-structure-test.yaml

# Scan for CVEs (required by security checklist)
trivy image --exit-code 1 --severity HIGH,CRITICAL myservice:1.0.0
```

## Key Details

- All final stages run as UID 65532 (nonroot) — Kubernetes `runAsNonRoot: true` will pass without extra config
- Distroless images have **no shell** — `HEALTHCHECK CMD` does not work; use Kubernetes httpGet probes instead
- Base image digests in the templates are placeholders (`sha256:000...001`) — pin real digests before production use (see README.md)
- Java template uses alpine JRE (has `wget`) — includes a `HEALTHCHECK` directive targeting `/actuator/health`
- Python uses `uv` (astral-sh) for fast dependency installation in the builder stage
- Java/Rust use `--mount=type=cache` — requires BuildKit (`DOCKER_BUILDKIT=1`)

## Related

- `tools/dockerize.py` — detects language, selects template, substitutes variables
- `services/python-api/Dockerfile` and `services/go-api/Dockerfile` — concrete examples
- Root `CLAUDE.md` → Security Checklist — trivy scan requirements
