# Cilium, Hubble & Tetragon: Network and Runtime Security for Chicklets

This document covers how to set up a full security observability and enforcement stack on a local Kind cluster using Cilium (CNI + network policy), Hubble (L7 network visibility), and Tetragon (eBPF runtime security).

## Table of Contents

- [Prerequisites](#prerequisites)
- [Architecture Overview](#architecture-overview)
- [Cluster Setup](#cluster-setup)
- [Demo 1: Runtime Security Observability](#demo-1-runtime-security-observability)
- [Demo 2: DNS Allowlisting](#demo-2-dns-allowlisting)
- [Demo 3: Pod-to-Pod Trust Groups](#demo-3-pod-to-pod-trust-groups)
- [Demo 4: Cross-Node Communication](#demo-4-cross-node-communication)
- [Demo 5: Cross-Namespace Org Isolation](#demo-5-cross-namespace-org-isolation)
- [TracingPolicy Reference](#tracingpolicy-reference)
- [CiliumNetworkPolicy Reference](#ciliumnetworkpolicy-reference)
- [Operational Commands](#operational-commands)

---

## Prerequisites

- Docker installed and running
- [kind](https://kind.sigs.k8s.io/) CLI
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm 3](https://helm.sh/docs/intro/install/)
- [Cilium CLI](https://docs.cilium.io/en/stable/gettingstarted/k8s-install-default/#install-the-cilium-cli)
- [Hubble CLI](https://docs.cilium.io/en/stable/gettingstarted/hubble_setup/#install-the-hubble-client)
- `chicklet-workspace:latest` Docker image built locally

```bash
# macOS install (if needed)
brew install kind kubectl helm cilium-cli hubble

# Build the chicklet image (from nightshift repo root)
docker build -t chicklet-workspace:latest ./tools/rootfs
```

---

## Architecture Overview

| Component | What it does | Scope |
|-----------|-------------|-------|
| **Cilium** | Replaces the default CNI. Enforces network policy at the eBPF level using identity-based rules (not IPs). | Cluster-wide |
| **Hubble** | Observability layer on Cilium. Aggregates L3/L4/L7 network flows from all nodes. Parses DNS queries into human-readable domain names. | Cluster-wide (via Hubble Relay) |
| **Tetragon** | eBPF runtime security. Hooks kernel functions to observe process execution, file access, network connections, and syscalls per-pod. | Per-node (DaemonSet) |

How they complement each other:

- **Tetragon** tells you *which process* inside a pod did something (e.g., `/usr/bin/curl` connected to `104.16.9.34:443`)
- **Hubble** tells you the *network-level verdict* (allowed/dropped) and resolves DNS to domain names
- **Cilium** *enforces* the policy in the kernel before packets hit the wire

---

## Cluster Setup

### 1. Create the Kind cluster

The cluster needs multiple worker nodes (for cross-node demos), the default CNI disabled (Cilium replaces it), and `/proc` mounted for Tetragon's eBPF programs.

```yaml
# kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true          # Cilium will be the CNI
  podSubnet: "10.244.0.0/16"      # Pod CIDR for Cilium to manage
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: /proc
        containerPath: /procHost   # Tetragon reads /proc via this mount
  - role: worker
    extraMounts:
      - hostPath: /proc
        containerPath: /procHost
  - role: worker
    extraMounts:
      - hostPath: /proc
        containerPath: /procHost
```

```bash
kind create cluster --config kind-config.yaml
```

### 2. Install Cilium with Hubble

```bash
cilium install --set hubble.relay.enabled=true --set hubble.ui.enabled=true
cilium status --wait
```

### 3. Install Tetragon

```bash
helm repo add cilium https://helm.cilium.io
helm repo update
helm install tetragon --set tetragon.hostProcPath=/procHost cilium/tetragon -n kube-system
kubectl rollout status -n kube-system ds/tetragon -w
```

### 4. Load the chicklet image into Kind

```bash
kind load docker-image chicklet-workspace:latest
```

### 5. Port-forward Hubble Relay

```bash
kubectl port-forward -n kube-system svc/hubble-relay 4245:80 &
```

---

## Demo 1: Runtime Security Observability

Deploy a single chicklet pod and observe process execution, network connections, and file access with Tetragon.

### Deploy

```bash
kubectl apply -f - <<'EOF'
---
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: v1
kind: Pod
metadata:
  name: cl-demo
  namespace: demo
  labels:
    app: chicklet
    chicklet: demo
spec:
  restartPolicy: Always
  containers:
    - name: chicklet
      image: chicklet-workspace:latest
      imagePullPolicy: Never
      command: ["/bin/bash", "-c"]
      args:
        - |
          mkdir -p /run/sshd && /usr/sbin/sshd
          python3 -m http.server 8080 &
          exec sleep infinity
      ports:
        - containerPort: 22
        - containerPort: 8080
      resources:
        requests: { cpu: "500m", memory: "512Mi" }
        limits:   { cpu: "1",    memory: "1Gi" }
---
# Monitor outbound TCP connections
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: monitor-tcp-connect
spec:
  kprobes:
    - call: "tcp_connect"
      syscall: false
      args:
        - index: 0
          type: "sock"
      selectors:
        - matchArgs:
            - index: 0
              operator: "NotDAddr"
              values:
                - "127.0.0.1"
---
# Monitor reads/writes to sensitive files
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: monitor-sensitive-files
spec:
  kprobes:
    - call: "security_file_permission"
      syscall: false
      return: true
      args:
        - index: 0
          type: "file"
        - index: 1
          type: "int"
      returnArg:
        index: 0
        type: "int"
      returnArgAction: "Post"
      selectors:
        - matchArgs:
            - index: 0
              operator: "Prefix"
              values: ["/etc/shadow", "/etc/passwd", "/root/.ssh"]
            - index: 1
              operator: "Equal"
              values: ["4"]
        - matchArgs:
            - index: 0
              operator: "Prefix"
              values: ["/etc", "/root/.ssh", "/var/log"]
            - index: 1
              operator: "Equal"
              values: ["2"]
EOF

kubectl wait --for=condition=Ready pod/cl-demo -n demo --timeout=60s
```

### Generate events

```bash
kubectl exec cl-demo -n demo -- bash -c 'curl -sS https://api.anthropic.com 2>&1 | head -c 80'
kubectl exec cl-demo -n demo -- bash -c 'cat /etc/shadow > /dev/null 2>&1'
kubectl exec cl-demo -n demo -- bash -c 'echo "test" >> /var/log/test.log'
kubectl exec cl-demo -n demo -- bash -c 'git ls-remote https://github.com/anthropics/claude-code.git HEAD 2>&1 | head -1'
kubectl exec cl-demo -n demo -- bash -c 'python3 -c "import socket; s=socket.socket(); s.connect((\"1.1.1.1\", 443)); s.close()"'
```

### Observe

```bash
# Tetragon: process + file + network events
kubectl exec -n kube-system ds/tetragon -c tetragon -- tetra getevents -o compact --namespace demo
```

Expected output:

```
🚀 process demo/cl-demo /usr/bin/curl -sS https://api.anthropic.com
🔌 connect demo/cl-demo /usr/bin/curl tcp 10.244.0.6:47320 -> 160.79.104.10:443
📚 read    demo/cl-demo /usr/bin/cat /etc/shadow
📝 write   demo/cl-demo /usr/bin/bash /var/log/test.log
🚀 process demo/cl-demo /usr/bin/git ls-remote https://github.com/anthropics/claude-code.git HEAD
🔌 connect demo/cl-demo /usr/lib/git-core/git-remote-https tcp ... -> 140.82.113.3:443
🔌 connect demo/cl-demo /usr/bin/python3 tcp ... -> 1.1.1.1:443
```

---

## Demo 2: DNS Allowlisting

Restrict chicklet pods to only connect to approved external domains. All other outbound traffic is denied. Hubble shows DNS queries with human-readable domain names.

### Deploy

Assumes Demo 1 namespace and pods exist. Apply the CiliumNetworkPolicy:

```bash
kubectl apply -f - <<'EOF'
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: dns-allowlist
  namespace: demo
spec:
  endpointSelector:
    matchLabels:
      app: chicklet
  egress:
    # Allow DNS resolution (activates Cilium DNS proxy for Hubble L7 visibility)
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: ANY
          rules:
            dns:
              - matchPattern: "*"
    # Allow HTTPS only to approved domains
    - toFQDNs:
        - matchName: "api.anthropic.com"
        - matchName: "api.openai.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
    # Everything else is implicitly denied once any egress rule exists
EOF
```

### Test

```bash
# Should PASS
kubectl exec cl-demo -n demo -- curl -sS --connect-timeout 5 https://api.anthropic.com 2>&1 | head -c 80

# Should FAIL (timeout)
kubectl exec cl-demo -n demo -- curl -sS --connect-timeout 5 https://github.com 2>&1 | head -1
kubectl exec cl-demo -n demo -- curl -sS --connect-timeout 5 https://registry.npmjs.org/express 2>&1 | head -1
kubectl exec cl-demo -n demo -- curl -sS --connect-timeout 5 https://example.com 2>&1 | head -1
```

### Observe

```bash
# Hubble: DNS queries with domain names
hubble observe --namespace demo --type l7 --protocol dns --last 20

# Hubble: dropped traffic
hubble observe --namespace demo --verdict DROPPED --last 20
```

Expected Hubble output:

```
demo/cl-demo -> kube-system/coredns  dns-request  FORWARDED  (DNS Query api.anthropic.com A)
demo/cl-demo -> kube-system/coredns  dns-response FORWARDED  (DNS Answer "160.79.104.10" TTL: 30)
demo/cl-demo <> github.com:443       EGRESS DENIED            (TCP Flags: SYN)
demo/cl-demo <> github.com:443       Policy denied DROPPED    (TCP Flags: SYN)
```

### Adding more allowed domains

Edit the policy and add entries to `toFQDNs`:

```yaml
    - toFQDNs:
        - matchName: "api.anthropic.com"
        - matchName: "api.openai.com"
        - matchName: "registry.npmjs.org"
        - matchPattern: "*.github.com"       # Wildcards supported
```

---

## Demo 3: Pod-to-Pod Trust Groups

Deploy three chicklets. Two share a trust group label and can communicate. The third is isolated.

### Deploy

```bash
kubectl apply -f - <<'EOF'
---
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
# Isolated chicklet (no trust-group label)
apiVersion: v1
kind: Pod
metadata:
  name: cl-isolated
  namespace: demo
  labels:
    app: chicklet
    chicklet: isolated
spec:
  nodeName: kind-worker
  restartPolicy: Always
  containers:
    - name: chicklet
      image: chicklet-workspace:latest
      imagePullPolicy: Never
      command: ["/bin/bash", "-c"]
      args:
        - |
          mkdir -p /run/sshd && /usr/sbin/sshd
          python3 -m http.server 8080 &
          exec sleep infinity
      ports:
        - containerPort: 8080
      resources:
        requests: { cpu: "250m", memory: "256Mi" }
        limits:   { cpu: "500m", memory: "512Mi" }
---
# Trust group member on worker1
apiVersion: v1
kind: Pod
metadata:
  name: cl-alpha
  namespace: demo
  labels:
    app: chicklet
    chicklet: alpha
    trust-group: shared
spec:
  nodeName: kind-worker
  restartPolicy: Always
  containers:
    - name: chicklet
      image: chicklet-workspace:latest
      imagePullPolicy: Never
      command: ["/bin/bash", "-c"]
      args:
        - |
          mkdir -p /run/sshd && /usr/sbin/sshd
          python3 -m http.server 8080 &
          exec sleep infinity
      ports:
        - containerPort: 8080
      resources:
        requests: { cpu: "250m", memory: "256Mi" }
        limits:   { cpu: "500m", memory: "512Mi" }
---
# Trust group member on worker2
apiVersion: v1
kind: Pod
metadata:
  name: cl-bravo
  namespace: demo
  labels:
    app: chicklet
    chicklet: bravo
    trust-group: shared
spec:
  nodeName: kind-worker2
  restartPolicy: Always
  containers:
    - name: chicklet
      image: chicklet-workspace:latest
      imagePullPolicy: Never
      command: ["/bin/bash", "-c"]
      args:
        - |
          mkdir -p /run/sshd && /usr/sbin/sshd
          python3 -m http.server 8080 &
          exec sleep infinity
      ports:
        - containerPort: 8080
      resources:
        requests: { cpu: "250m", memory: "256Mi" }
        limits:   { cpu: "500m", memory: "512Mi" }
---
# DNS allowlist for all chicklets
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: dns-allowlist
  namespace: demo
spec:
  endpointSelector:
    matchLabels:
      app: chicklet
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: ANY
          rules:
            dns:
              - matchPattern: "*"
    - toFQDNs:
        - matchName: "api.anthropic.com"
        - matchName: "api.openai.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
---
# Trust group: pods with trust-group=shared can talk to each other
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: shared-group
  namespace: demo
spec:
  endpointSelector:
    matchLabels:
      trust-group: shared
  ingress:
    - fromEndpoints:
        - matchLabels:
            trust-group: shared
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: ANY
          rules:
            dns:
              - matchPattern: "*"
    - toEndpoints:
        - matchLabels:
            trust-group: shared
    - toFQDNs:
        - matchName: "api.anthropic.com"
        - matchName: "api.openai.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
EOF

kubectl wait --for=condition=Ready pods --all -n demo --timeout=60s
```

### Test

```bash
ALPHA_IP=$(kubectl get pod cl-alpha -n demo -o jsonpath='{.status.podIP}')
BRAVO_IP=$(kubectl get pod cl-bravo -n demo -o jsonpath='{.status.podIP}')
ISOLATED_IP=$(kubectl get pod cl-isolated -n demo -o jsonpath='{.status.podIP}')

# Trust group members can communicate (cross-node)
kubectl exec cl-alpha -n demo -- curl -sS --connect-timeout 3 http://$BRAVO_IP:8080 | head -1  # PASS
kubectl exec cl-bravo -n demo -- curl -sS --connect-timeout 3 http://$ALPHA_IP:8080 | head -1  # PASS

# Isolated pod cannot reach trust group members
kubectl exec cl-isolated -n demo -- curl -sS --connect-timeout 3 http://$ALPHA_IP:8080 2>&1 | head -1  # DENIED
kubectl exec cl-isolated -n demo -- curl -sS --connect-timeout 3 http://$BRAVO_IP:8080 2>&1 | head -1  # DENIED

# Trust group members cannot reach isolated pod
kubectl exec cl-bravo -n demo -- curl -sS --connect-timeout 3 http://$ISOLATED_IP:8080 2>&1 | head -1  # DENIED
```

### Observe

```bash
# All drops
hubble observe --namespace demo --verdict DROPPED --last 20

# Cilium identity assignments
kubectl exec -n kube-system ds/cilium -- cilium identity list | grep demo
```

### How trust groups work

"Trust group" is not a Cilium concept. It is an arbitrary Kubernetes label (`trust-group: shared`) that the CiliumNetworkPolicy selects on. The policy says: pods with this label can talk to other pods with this label. Cilium resolves labels to numeric identities and enforces at the eBPF level, not by IP. A pod restart or IP change does not break the policy.

You can name the label anything: `org: acme`, `team: backend`, `project: foo`.

---

## Demo 4: Cross-Node Communication

This is built into Demo 3 above. `cl-alpha` runs on `kind-worker` and `cl-bravo` runs on `kind-worker2`. Their communication crosses the Cilium VXLAN/Geneve overlay tunnel.

### Verify pod placement

```bash
kubectl get pods -n demo -o custom-columns='POD:.metadata.name,NODE:.spec.nodeName,IP:.status.podIP'
```

### Observe cross-node flows

```bash
# Hubble shows "to-overlay" for traffic crossing nodes
hubble observe --namespace demo --verdict FORWARDED --last 20
```

Look for `to-overlay FORWARDED` — this means the packet traversed the inter-node tunnel:

```
cl-alpha:45088 -> cl-bravo:8080  to-overlay FORWARDED (TCP Flags: SYN)     # leaves worker
cl-alpha:45088 -> cl-bravo:8080  to-endpoint FORWARDED (TCP Flags: SYN)    # arrives worker2
```

### Tetragon per-node events

Tetragon runs as a DaemonSet. Each node's agent only sees its own pods. Query a specific node:

```bash
# Events from worker (cl-alpha + cl-isolated)
TETRA_W1=$(kubectl get pods -n kube-system -l app.kubernetes.io/name=tetragon \
  --field-selector spec.nodeName=kind-worker -o name | head -1)
kubectl exec -n kube-system $TETRA_W1 -c tetragon -- tetra getevents -o compact --namespace demo

# Events from worker2 (cl-bravo)
TETRA_W2=$(kubectl get pods -n kube-system -l app.kubernetes.io/name=tetragon \
  --field-selector spec.nodeName=kind-worker2 -o name | head -1)
kubectl exec -n kube-system $TETRA_W2 -c tetragon -- tetra getevents -o compact --namespace demo
```

Key insight: Tetragon sees the `connect()` syscall on the source node even when Cilium drops the packet. The process still tried. Hubble shows whether it was allowed or dropped on the wire.

---

## Demo 5: Cross-Namespace Org Isolation

Deploy two separate orgs in their own namespaces. Each org's chicklets can only communicate within their namespace. Cross-namespace traffic is denied.

### Deploy

```bash
kubectl apply -f - <<'EOF'
---
apiVersion: v1
kind: Namespace
metadata:
  name: org-acme
---
apiVersion: v1
kind: Namespace
metadata:
  name: org-globex
---
# ACME chicklet on worker1
apiVersion: v1
kind: Pod
metadata:
  name: cl-acme-dev
  namespace: org-acme
  labels:
    app: chicklet
    chicklet: acme-dev
    org: acme
spec:
  nodeName: kind-worker
  restartPolicy: Always
  containers:
    - name: chicklet
      image: chicklet-workspace:latest
      imagePullPolicy: Never
      command: ["/bin/bash", "-c"]
      args:
        - |
          mkdir -p /run/sshd && /usr/sbin/sshd
          python3 -m http.server 8080 &
          exec sleep infinity
      ports:
        - containerPort: 8080
      resources:
        requests: { cpu: "250m", memory: "256Mi" }
        limits:   { cpu: "500m", memory: "512Mi" }
---
# Globex chicklet on worker2
apiVersion: v1
kind: Pod
metadata:
  name: cl-globex-dev
  namespace: org-globex
  labels:
    app: chicklet
    chicklet: globex-dev
    org: globex
spec:
  nodeName: kind-worker2
  restartPolicy: Always
  containers:
    - name: chicklet
      image: chicklet-workspace:latest
      imagePullPolicy: Never
      command: ["/bin/bash", "-c"]
      args:
        - |
          mkdir -p /run/sshd && /usr/sbin/sshd
          python3 -m http.server 8080 &
          exec sleep infinity
      ports:
        - containerPort: 8080
      resources:
        requests: { cpu: "250m", memory: "256Mi" }
        limits:   { cpu: "500m", memory: "512Mi" }
---
# ACME: only talk within org-acme namespace
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: org-isolation
  namespace: org-acme
spec:
  endpointSelector:
    matchLabels:
      app: chicklet
  ingress:
    - fromEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: org-acme
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: ANY
          rules:
            dns:
              - matchPattern: "*"
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: org-acme
    - toFQDNs:
        - matchName: "api.anthropic.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
---
# Globex: only talk within org-globex namespace
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: org-isolation
  namespace: org-globex
spec:
  endpointSelector:
    matchLabels:
      app: chicklet
  ingress:
    - fromEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: org-globex
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: ANY
          rules:
            dns:
              - matchPattern: "*"
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: org-globex
    - toFQDNs:
        - matchName: "api.anthropic.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
EOF

kubectl wait --for=condition=Ready pod/cl-acme-dev -n org-acme --timeout=60s
kubectl wait --for=condition=Ready pod/cl-globex-dev -n org-globex --timeout=60s
```

### Test

```bash
ACME_IP=$(kubectl get pod cl-acme-dev -n org-acme -o jsonpath='{.status.podIP}')
GLOBEX_IP=$(kubectl get pod cl-globex-dev -n org-globex -o jsonpath='{.status.podIP}')

# Cross-namespace: both directions denied
kubectl exec cl-acme-dev -n org-acme -- curl -sS --connect-timeout 3 http://$GLOBEX_IP:8080 2>&1 | head -1    # DENIED
kubectl exec cl-globex-dev -n org-globex -- curl -sS --connect-timeout 3 http://$ACME_IP:8080 2>&1 | head -1  # DENIED

# External: both orgs can reach approved domains
kubectl exec cl-acme-dev -n org-acme -- curl -sS --connect-timeout 5 https://api.anthropic.com 2>&1 | head -c 80     # PASS
kubectl exec cl-globex-dev -n org-globex -- curl -sS --connect-timeout 5 https://api.anthropic.com 2>&1 | head -c 80  # PASS

# Cross-namespace from demo namespace also denied
kubectl exec cl-alpha -n demo -- curl -sS --connect-timeout 3 http://$ACME_IP:8080 2>&1 | head -1  # DENIED
```

### Observe

```bash
hubble observe --verdict DROPPED --last 20 | grep -E "acme|globex"
```

Expected:

```
org-acme/cl-acme-dev   <> org-globex/cl-globex-dev:8080  EGRESS DENIED (TCP Flags: SYN)
org-globex/cl-globex-dev <> org-acme/cl-acme-dev:8080    EGRESS DENIED (TCP Flags: SYN)
```

### How this maps to nightshift

Nightshift already creates a namespace per org (`org-<slug>`) or per user (`user-<id>`). Applying one `CiliumNetworkPolicy` per namespace (templated from the pattern above, substituting the namespace name) gives full org isolation. Pods in different orgs get different Cilium identities, so traffic is dropped at the eBPF level before hitting the wire — even on the same node.

---

## TracingPolicy Reference

### How kprobes work

A `kprobe` hooks an internal kernel function. Tetragon attaches an eBPF program to the function entry (and optionally return), captures the arguments, and filters using selectors.

```yaml
kprobes:
  - call: "function_name"    # Kernel function to hook
    syscall: false           # false = internal function, true = syscall
    return: true             # Also capture return value
    args:
      - index: 0             # Positional argument
        type: "sock"         # Type determines what Tetragon extracts
    returnArg:
      index: 0
      type: "int"
    returnArgAction: "Post"  # Emit event after function returns
    selectors:               # Filter which invocations emit events
      - matchArgs: [...]     # Filters within a selector are AND'd
      - matchArgs: [...]     # Multiple selectors are OR'd (max 5)
```

### Argument types

| Type | What Tetragon extracts |
|------|----------------------|
| `sock` | Source/dest IP, port, protocol, family |
| `file` | File path (from `struct file *`) |
| `path` | File path (from `struct path *`) |
| `int` / `uint32` | Integer value |
| `bytes` | Raw byte buffer |
| `skb` | Packet headers (L3/L4) |

### Selector operators

| Operator | Matches | Example |
|----------|---------|---------|
| `Equal` | Exact value | `values: ["4"]` matches `4` only |
| `NotEqual` | Not this value | |
| `Prefix` | String starts with | `values: ["/etc"]` matches `/etc/shadow` |
| `Postfix` | String ends with | `values: [".bashrc"]` matches `/home/user/.bashrc` |
| `Mask` | Bitwise AND is nonzero | `values: ["2"]` matches `3` (0x03 & 0x02 = 0x02) |
| `DAddr` / `NotDAddr` | Destination IP matches/doesn't match | `values: ["127.0.0.1"]` |
| `SAddr` / `NotSAddr` | Source IP | |
| `DPort` / `NotDPort` | Destination port | `values: ["53"]` |
| `SPort` / `NotSPort` | Source port | |
| `Protocol` | IP protocol | `values: ["IPPROTO_TCP"]` |

### Selector actions

| Action | Effect |
|--------|--------|
| `Post` | Emit event (default) |
| `Sigkill` | Kill the process immediately |
| `Signal` | Send a signal to the process |
| `Override` | Override the return value (block the operation) |

### Common hooks

| Hook | What it catches |
|------|----------------|
| `tcp_connect` | Outbound TCP connections |
| `security_file_permission` | File read/write permission checks |
| `security_mmap_file` | Memory-mapped file access (bypasses normal read/write) |
| `security_path_truncate` | File truncation (wiping logs, clearing files) |
| `udp_sendmsg` | Outbound UDP (including DNS) |
| `ip_output` | All outbound IP packets |

---

## CiliumNetworkPolicy Reference

### Key concepts

- **Identity-based**: Cilium assigns a numeric identity to each pod based on its labels. Policy is enforced against identities, not IPs. Pod restarts and IP changes don't break policy.
- **Default deny**: Once any egress (or ingress) rule exists for a selector, all traffic not matching a rule is denied.
- **DNS proxy**: The `rules.dns` block activates Cilium's DNS proxy, which intercepts DNS queries. This is required for `toFQDNs` rules and enables Hubble L7 DNS visibility.
- **Policy union**: If multiple policies match a pod (via different label selectors), the pod gets the union of all allowed traffic from all matching policies.

### Policy template for nightshift org isolation

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: org-isolation
  namespace: ${NAMESPACE}           # org-<slug> or user-<id>
spec:
  endpointSelector:
    matchLabels:
      app: chicklet
  ingress:
    - fromEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: ${NAMESPACE}
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: ANY
          rules:
            dns:
              - matchPattern: "*"
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: ${NAMESPACE}
    - toFQDNs:
        - matchName: "api.anthropic.com"
        # Add more allowed domains here
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
```

---

## Operational Commands

### Hubble

```bash
# DNS queries (human-readable domain names)
hubble observe --namespace demo --type l7 --protocol dns

# All flows for a namespace
hubble observe --namespace demo

# Only dropped traffic
hubble observe --namespace demo --verdict DROPPED

# Follow live
hubble observe --namespace demo --follow

# Launch Hubble UI (web dashboard)
cilium hubble ui
# Opens http://localhost:12000
```

### Tetragon

```bash
# Compact events for a namespace
kubectl exec -n kube-system ds/tetragon -c tetragon -- tetra getevents -o compact --namespace demo

# Full JSON (for shipping to Loki/SIEM)
kubectl exec -n kube-system ds/tetragon -c tetragon -- tetra getevents --namespace demo

# Events from a specific node
TETRA_POD=$(kubectl get pods -n kube-system -l app.kubernetes.io/name=tetragon \
  --field-selector spec.nodeName=kind-worker -o name | head -1)
kubectl exec -n kube-system $TETRA_POD -c tetragon -- tetra getevents -o compact --namespace demo

# List active tracing policies
kubectl get tracingpolicies
```

### Cilium

```bash
# Cluster status
cilium status

# Endpoint list with identities
kubectl exec -n kube-system ds/cilium -- cilium endpoint list

# Identity to label mapping
kubectl exec -n kube-system ds/cilium -- cilium identity list

# List network policies
kubectl get ciliumnetworkpolicies -A

# Policy troubleshooting for a specific endpoint
kubectl exec -n kube-system ds/cilium -- cilium endpoint get <endpoint-id>
```

### Cleanup

```bash
kind delete cluster --name kind
```
