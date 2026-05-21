# nightshift-worker-claude

Python container image for the reference LLM worker. Drives the [Claude
Agent SDK](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code-sdk)
and speaks the Worker-to-Platform protocol defined in
[`protos/nightshift/v1/worker-protocol.md`](../../protos/nightshift/v1/worker-protocol.md).

This image rides **alongside** the Go simulated worker at
`cmd/nightshift-worker/`, not as a replacement. Operators flip the
chart's `nightshift_api.worker.repository` value to switch which image
the API launches.

## Origin

Direct port of the production cr0n-a worker (`worker.py` /
`serialization.py` / `artifact_tools.py` / `bao_client.py` /
`report_generators.py`) with these adaptations:

- HTTP endpoints rebased onto nightshift's grpc-gateway URL layout
  (`/v1/internal/runs/{id}/events`, `:complete`, `:fail`,
  `/cancellation`, `/v1/users/{id}/config`).
- `X-Worker-Secret` header → `Authorization: Bearer
  <NS_WORKER_CREDENTIAL>` (HMAC-signed, scoped to RUN_ID per chunk 8c).
- cr0n env vars (`RUN_ID`, `API_CALLBACK_URL`, …) → `NS_*` (chunk 8b).
- cr0n `RUN_CLAUDE_SESSION_ID` → `NS_SDK_SESSION_ID`, populated by the
  API from the prior run's `attrSDKSessionID` Record attribute. The
  SDK id never crosses the outer (user-facing) gRPC surface
  (workers.md §4).
- `<workspace>/.claude/projects` is symlinked to `NS_SESSION_STATE_DIR`
  on startup so the SDK's JSONL transcripts persist across runs of the
  same platform session (rides on chunk 13's per-session volume).

## Environment contract

Required (must be set by the launcher; see `internal/runtime/`):

| Var | Purpose |
|---|---|
| `NS_RUN_ID` | The run id the worker reports back against |
| `NS_PROMPT` | The user / scheduler prompt to drive the agent |
| `NS_API_URL` | Base URL of the nightshift-api **HTTP gateway** (e.g. `http://nightshift-nightshift-api.nightshift.svc:8080`) — **not** the gRPC port; this Python worker speaks REST. The chart sets this automatically when `workerClaude.enabled`. |
| `NS_WORKER_CREDENTIAL` | HMAC bearer (chunk 8c). Authorizes the inner-surface RPCs only. |

Optional:

| Var | Default | Purpose |
|---|---|---|
| `NS_USER_ID` | empty | Owning user; when set, the worker fetches Config Dispenser data |
| `NS_SESSION_ID` | empty | Platform-owned session id (chunk 13) |
| `NS_SDK_SESSION_ID` | empty | SDK-internal session id for resume; injected by the API on follow-up runs |
| `NS_SESSION_STATE_DIR` | empty | Per-session persistent dir (chunk 13). When set, SDK transcripts live here. |
| `NS_WORKSPACE` | `/home/nightshift/workspace` | Per-pod ephemeral workspace; agent `cwd` |
| `NS_OPENBAO_ADDR` | `http://openbao.nightshift.svc:8200` | OpenBao service address |
| `NS_OPENBAO_AUTH_ROLE` | `nightshift-worker` | K8s-auth role bound to the worker SA |
| `NS_ANTHROPIC_KEY_PATH` | `nightshift/anthropic-api-key` | KV path that holds `api-key` |
| `NS_CANCEL_POLL_INTERVAL` | `5` | Poll cancellation every N events |

The worker reads `ANTHROPIC_API_KEY` by logging into OpenBao with the
pod's K8s SA token, reading `secret/data/<NS_ANTHROPIC_KEY_PATH>`, and
exporting the `api-key` field as `ANTHROPIC_API_KEY` for the SDK.

## MCP tool inventory

The worker bakes in 11 MCP tools from cr0n's `artifact_tools.py`. **Most
of these depend on backend endpoints that don't exist yet** — the
agent will see runtime errors when it invokes them until those chunks
land. Inventory:

| Tool | Backend chunk | Status today |
|---|---|---|
| `deploy_app` | 16 | ❌ broken until chunk 16 |
| `deploy_object` | 15 | ❌ broken until chunk 15 |
| `list_artifacts` | 15 | ❌ broken until chunk 15 |
| `update_artifact` | 15 | ❌ broken until chunk 15 |
| `share_artifact` | 15 | ❌ broken until chunk 15 |
| `show_preview_artifact` | n/a (UI-side) | ✅ no-op handler — UI reads from `tool_use.input` |
| `create_pdf` / `create_docx` / `create_xlsx` / `create_pptx` | 15 (uses `deploy-object`) | ❌ broken until chunk 15 |
| `create_schedule` | 17 | ❌ broken until chunk 17 |

When chunks 15-17 land, those backends light up and the tools start
working without any changes to this image.

## SDK session resume — composition with chunk 13

The chunk-14 control plane handles the platform↔SDK session-id bridge:

1. Worker reports the SDK's id via `CompleteRunRequest.session_id` on its way out.
2. API persists it as the `attrSDKSessionID` Record attribute on the Run.
3. On a follow-up `CreateRun` with the same platform `session_id`, the API
   looks up the most recent terminal run's attribute and injects
   `NS_SDK_SESSION_ID` onto the new worker pod.
4. The worker passes that value to `ClaudeAgentOptions(resume=…)`.

But the SDK's actual conversation transcript (the `.jsonl` files under
`~/.claude/projects/`) lives on the worker pod's filesystem. Worker
pods are ephemeral, so without a persistent mount the transcript dies
with the pod and `--resume <id>` fails with `No conversation found`.

**The fix is chunk 13.** When `nightshift_api.sessionState.backend` is
set to a real persistent backend, the worker symlinks
`<workspace>/.claude/projects` → `NS_SESSION_STATE_DIR` on startup so
the SDK's transcripts persist across runs of the same session.

| backend | resume on kind | resume on EKS |
|---|---|---|
| `none` (default) | ❌ no | ❌ no |
| `host` | ✅ single-node only | ❌ — multi-node clusters can't share hostPaths |
| `pvc` | ❌ — kind's `local-path` is RWO; PVC stays Pending | ✅ with EFS / FSx for Lustre / similar RWX CSI driver |
| `object` (chunk 13 cascade-only) | ❌ — worker round-trip lands in chunk 15+ | ❌ same |

For the production deployment on EKS, enable
`nightshift_api.sessionState.backend=pvc` with a `ReadWriteMany`-capable
storage class (EFS is the canonical choice). On kind, the chunk-14
control-plane resume path can be verified end-to-end by inspecting
`NS_SDK_SESSION_ID` on the resume worker pod's env (it lands correctly),
but the SDK-level resume requires the chunk-13 mount.

## Building locally

```bash
make docker-claude            # builds ghcr.io/nightshiftco/nightshift-worker-claude:dev
make kind-load-claude         # also loads it into the kind cluster
```

## Deploying

Requires `openbao.enabled=true` (for K8s-auth + KV reads) and the
operator pre-seeding the API key:

```bash
# Pre-seed the key (once per cluster):
kubectl -n nightshift exec sts/nightshift-openbao -- \
  bao kv put secret/nightshift/anthropic-api-key api-key=sk-ant-xxxxx

# Flip the worker image and enable the workerClaude policy extension:
helm upgrade --install nightshift deploy/charts/nightshift \
  --reuse-values \
  --set nightshift_api.worker.repository=nightshift-worker-claude \
  --set nightshift_api.workerClaude.enabled=true
```

Then `CreateRun` with a real prompt; the worker will fetch the API key
from OpenBao at startup, drive the SDK against Claude, post events
back, and complete with usage + the SDK session id (which the API
stashes for resume on the next run with the same `session_id`).
