# agents/nightshift/

Go monorepo for the Nightshift cost-optimization scheduler — scales workloads to zero on idle and back on demand via a gRPC/HTTP API and worker protocol.

## What's Here

```
cmd/
  nightshift-api/         # gRPC + grpc-gateway HTTP server (port 8080)
  nightshift-ui/          # Web UI binary
  nightshift-worker/      # Reference worker — proves agent-to-platform protocol (no LLM calls)
  nightshift-worker-claude/ # Claude-powered worker variant
internal/
  api/                    # Service implementations (artifacts, auth, config, records, runtime, secrets)
  broadcaster/            # Event broadcast layer
  identity/               # JWT / OIDC identity
  metrics/                # Prometheus instrumentation
protos/                   # Protobuf definitions (buf-managed)
gen/                      # Generated Go + gRPC code (buf generate)
deploy/                   # Kubernetes/Helm deployment manifests (argocd, eks, kind, monitoring)
```

## How to Use / Run

```bash
# Generate protobuf code (requires buf)
buf generate

# Build all binaries
go build ./cmd/...

# Run the API server locally
go run ./cmd/nightshift-api

# Run the reference worker (simulation only, no LLM)
go run ./cmd/nightshift-worker

# Run tests
go test ./...
```

## Key Details

- **Module**: `github.com/nightshiftco/nightshift`
- **Go version**: 1.25+
- **Key deps**: `grpc-gateway/v2`, `pgx/v5` (Postgres), `aws-sdk-go-v2`, `prometheus/client_golang`, `testcontainers-go`
- The worker protocol is documented at `protos/nightshift/v1/worker-protocol.md`
- `nightshift-worker` is intentionally simulation-only — it drives the full protocol state machine (CreateRun → StreamRunEvents → COMPLETED) without calling any LLM
- `nightshift-worker-claude` is the production variant that calls Claude via the Anthropic API
- Database: PostgreSQL (pgx driver); `testcontainers-go` spins up a real PG container for integration tests
- Storage: AWS S3 compatible (uses gofakes3 for tests)

## Related

- `templates/keda/` — KEDA HTTPScaledObject is the Kubernetes complement (immediate scale-to-zero); nightshift handles scheduled/policy-driven scale-down
- `tools/cost_optimize.py` — applies KEDA manifests to a cluster; nightshift provides the scheduling API on top
- `deploy/eks/` and `deploy/kind/` — cluster-specific deployment configurations
