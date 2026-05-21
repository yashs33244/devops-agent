# Cilium NetworkPolicies for nightshift workers

These `CiliumNetworkPolicy` (CNP) manifests sandbox the network surface
of nightshift worker pods — the highest-privilege surface in the
control plane, since they execute untrusted agent-generated code.

## What's in here

| File | Effect |
|---|---|
| `00-default-deny.yaml` | Empty ingress/egress on the worker selector. Deny-all baseline. |
| `10-allow-dns.yaml` | Allow DNS to kube-dns on port 53 (with L7 query logging). |
| `20-allow-api-callback.yaml` | Allow egress to `nightshift-api` pods on ports 50051 (gRPC) + 8080 (HTTP). |

CNPs are **additive**: any allow rule is a union, but the default-deny
remains in force for everything not explicitly allowed.

## Selector

All policies match worker pods by:

```yaml
app.kubernetes.io/name: nightshift
app.kubernetes.io/component: worker
```

These labels are emitted by the launcher (`internal/runtime/kubernetes.go`)
on every `Job`/`Pod` it creates. Do not gate selection on
`app.kubernetes.io/instance` — that would tie the policy to a specific
helm release name.

## Prerequisites

- A Cilium-managed cluster (`cilium status` reports OK).
- For the L7 DNS rules in `10-allow-dns.yaml`, Cilium must be installed
  with DNS policy enforcement enabled (default for v1.16+).

## Install

Standalone:

```sh
kubectl apply -n <nightshift-namespace> -f deploy/cilium/
```

Via the Helm chart:

```sh
helm install nightshift deploy/charts/nightshift \
  --set cilium.enabled=true \
  ...
```

## Verify

End-to-end recipe (probe pod + Hubble verdicts + Tetragon audit
readout) lives in
[`deploy/charts/nightshift/README.md` § *Verifying enforcement on kind*](../charts/nightshift/README.md#verifying-enforcement-on-kind).

In short: deploy a pod with the worker labels and confirm
`nc nightshift-api 50051` succeeds, `curl https://1.1.1.1` times out,
and `cilium hubble observe --pod nightshift/<probe>` shows
`policy-verdict:L3-L4 EGRESS ALLOWED` on the API port and
`Policy denied DROPPED` on world IPs.

## Forward note for chunk 14

`nightshift-worker-claude` (the real LLM worker) needs egress to
provider APIs (e.g. `api.anthropic.com:443`). That allow lands as a
new sibling CNP — `30-allow-anthropic-fqdn.yaml` or similar — when
chunk 14 ships. **Do not relax `00-default-deny.yaml`** to make room;
the model is additive allows.
