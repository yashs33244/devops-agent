# Production Dockerfile Templates

Multi-stage, minimal, non-root container images for common application runtimes.
Each template follows Docker and container-security best practices: pinned digests,
cache-efficient layer ordering, distroless or minimal final images, and non-root users.

---

## Image Overview

| Language | Builder base             | Final base                                  | Approx final size |
|----------|--------------------------|---------------------------------------------|-------------------|
| Node.js  | `node:22-alpine`         | `gcr.io/distroless/nodejs22-debian12:nonroot`| ~120 MB           |
| Python   | `python:3.12-slim-bookworm` | `gcr.io/distroless/python3-debian12:nonroot` | ~60 MB           |
| Go       | `golang:1.23-alpine`     | `gcr.io/distroless/static-debian12:nonroot` | ~5–15 MB          |
| Java     | `eclipse-temurin:21-jdk-alpine` | `eclipse-temurin:21-jre-alpine`       | ~220 MB           |
| Rust     | `rust:1.82-alpine`       | `gcr.io/distroless/static-debian12:nonroot` | ~5–15 MB          |

> Sizes are approximate and depend on dependencies. Distroless static images (Go, Rust)
> are particularly small because no runtime interpreter is needed.

---

## Template Variables

Before building, replace all `{{VARNAME}}` placeholders in each Dockerfile:

| Variable        | Description                                    | Default |
|-----------------|------------------------------------------------|---------|
| `{{PORT}}`      | Port the application listens on                | varies  |
| `{{BINARY_NAME}}` | Output binary name (Go, Rust)                | `server`|
| `{{MAIN_MODULE}}` | Python uvicorn module path (e.g. `app.main:app`) | —   |

---

## Building Images

Always target the `runner` stage to avoid shipping build tools:

```bash
# Generic pattern
docker build --target runner -t myservice:1.0.0 .

# Node.js
docker build --target runner -t my-node-app:1.0.0 node/

# Python
docker build --target runner -t my-python-app:1.0.0 python/

# Go (set BINARY_NAME as a build arg if you customised the Dockerfile)
docker build --target runner -t my-go-service:1.0.0 go/

# Java
docker build --target runner -t my-java-service:1.0.0 java/

# Rust
docker build --target runner -t my-rust-service:1.0.0 rust/
```

### BuildKit cache mounts (Java, Rust)

The Java and Rust Dockerfiles use `--mount=type=cache` and the dependency-caching
stub pattern respectively. These require BuildKit:

```bash
DOCKER_BUILDKIT=1 docker build --target runner -t my-java-service:1.0.0 java/
# Or use Docker Desktop / recent Docker Engine which enables BuildKit by default.
```

---

## Running Container Structure Tests

[container-structure-test](https://github.com/GoogleContainerTools/container-structure-test)
validates the image contents, metadata, and runtime behaviour without starting
the actual application.

### Install

```bash
# macOS
brew install container-structure-test

# Linux
curl -LO https://storage.googleapis.com/container-structure-test/latest/container-structure-test-linux-amd64
chmod +x container-structure-test-linux-amd64
sudo mv container-structure-test-linux-amd64 /usr/local/bin/container-structure-test
```

### Run tests

```bash
container-structure-test test \
  --image myservice:1.0.0 \
  --config node/container-structure-test.yaml

container-structure-test test \
  --image myservice:1.0.0 \
  --config python/container-structure-test.yaml

container-structure-test test \
  --image myservice:1.0.0 \
  --config go/container-structure-test.yaml
```

---

## Scanning for Vulnerabilities

Scan the final image before pushing to a registry:

```bash
# Docker Scout (requires Docker Desktop or docker scout plugin)
docker scout cves myservice:1.0.0

# Trivy (open-source, recommended for CI)
trivy image --exit-code 1 --severity HIGH,CRITICAL myservice:1.0.0

# Grype
grype myservice:1.0.0
```

Integrate into CI by adding the scan step after `docker build` and before `docker push`.

---

## Digest Pinning

All Dockerfiles reference base images by `@sha256:` digest (currently set to
placeholder `000...001` through `000...009`). Pinning by digest guarantees that
pulling the image at any future date uses exactly the same layer tree, regardless
of tag mutations.

### Obtaining real digests

```bash
# Pull the image and print the full digest
docker pull node:22-alpine --no-trunc
# Output: node:22-alpine@sha256:<actual-digest>

docker pull gcr.io/distroless/nodejs22-debian12:nonroot --no-trunc
docker pull python:3.12-slim-bookworm --no-trunc
docker pull gcr.io/distroless/python3-debian12:nonroot --no-trunc
docker pull golang:1.23-alpine --no-trunc
docker pull gcr.io/distroless/static-debian12:nonroot --no-trunc
docker pull eclipse-temurin:21-jdk-alpine --no-trunc
docker pull eclipse-temurin:21-jre-alpine --no-trunc
docker pull rust:1.82-alpine --no-trunc
```

Replace the placeholder digests in the `FROM` lines of each Dockerfile with the
values printed above. Re-pin periodically (e.g. monthly) to pick up security patches.

---

## Kubernetes Health Probes

Distroless images (Node, Python, Go, Rust) have no shell, so `HEALTHCHECK CMD`
is not supported. Configure probes at the Kubernetes deployment level instead:

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 15
  periodSeconds: 20
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

The Java template uses `eclipse-temurin:21-jre-alpine` (has `wget`) so a
`HEALTHCHECK` directive targeting `/actuator/health` is included directly.

---

## Non-Root Users

| Template | User      | UID   | How configured                          |
|----------|-----------|-------|-----------------------------------------|
| Node.js  | nonroot   | 65532 | Built into distroless image             |
| Python   | nonroot   | 65532 | Built into distroless image             |
| Go       | nonroot   | 65532 | Built into distroless image             |
| Java     | appuser   | dynamic | `addgroup -S appgroup && adduser -S appuser -G appgroup` |
| Rust     | nonroot   | 65532 | Built into distroless image             |

Never run containers as root in production. All templates enforce this.
