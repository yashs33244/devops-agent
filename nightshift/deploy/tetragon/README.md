# Tetragon TracingPolicies for nightshift workers

Syscall-level audit policies for worker pods. Worker pods execute
untrusted agent-generated code, so every exec, sensitive-file access,
and outbound TCP connect is logged for forensics and intrusion
detection.

All policies are **audit-only** (`Post` action). No `Sigkill`. Operators
who want enforcement layer it on once they've calibrated against their
LLM worker's expected behavior.

## What's in here

| File | Audits |
|---|---|
| `worker-exec-audit.yaml` | Every `execve` from a worker pod. Reference worker emits zero — any event is anomalous. |
| `worker-file-audit.yaml` | Reads of `/etc/shadow`, `/etc/passwd`, `/root/.ssh/`, the SA token dir. Writes to `/etc/` and `/var/log/`. |
| `worker-network-audit.yaml` | Non-loopback `tcp_connect`. Cilium drops unauthorized destinations at the packet level; this is the syscall-level audit trail. |

## Selector

All policies scope to:

```yaml
app.kubernetes.io/name: nightshift
app.kubernetes.io/component: worker
```

Policies are cluster-scoped (`TracingPolicy`, not `TracingPolicyNamespaced`)
so they cover worker pods regardless of which namespace they run in.

## Prerequisites

- Tetragon installed cluster-wide (`kubectl get pods -n kube-system -l app.kubernetes.io/name=tetragon`).
- Tetragon's stdout exporter or runtime hooks enabled — events are useless if no sink consumes them.

## Install

Standalone:

```sh
kubectl apply -f deploy/tetragon/
```

Via the Helm chart:

```sh
helm install nightshift deploy/charts/nightshift \
  --set tetragon.enabled=true \
  ...
```

## View events

```sh
kubectl logs -n kube-system -l app.kubernetes.io/name=tetragon \
  -c export-stdout --tail=100 -f
```

Filter by run-id:

```sh
kubectl logs ... | jq 'select(.process_kprobe.process.pod.labels[]? | contains("run-id="))'
```

End-to-end verification recipe (cluster install, probe pod, expected
event shapes) lives in
[`deploy/charts/nightshift/README.md` § *Verifying enforcement on kind*](../charts/nightshift/README.md#verifying-enforcement-on-kind).

## Threat model

See `deploy/THREAT_MODEL.md` for the full per-policy rationale.
