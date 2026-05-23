# /kagent — Kubernetes-Native AI Agent Framework

kagent is a Kubernetes-native AI agent framework that runs AI agents as pods inside a K8s cluster. It manages agents via CRDs (Agent, AgentConfig, Tool, ModelConfig). Lives at `agents/kagent/`. Main controller: `agents/kagent/go/core/cmd/controller/main.go`.

## Step 1: Ask What to Do

Offer the following options:

1. List agents in the cluster
2. Create a new agent
3. Delete an agent
4. Run a task on an existing agent
5. View agent logs
6. Check agent status
7. Deploy kagent controller to cluster

## Step 2: Collect Operation-Specific Inputs

**For "list agents":** ask for namespace (default: `default`).

**For "create agent":** ask for:
- Agent name (lowercase, hyphens only)
- Model backend: `claude` / `openai` / `gemini`
- Tools to give it (e.g. `kubectl`, `helm`, `prometheus`, `custom`)
- Namespace

**For "run task":** ask for:
- Agent name
- Task description (natural language)
- Namespace

**For "view logs" / "check status":** ask for agent name and namespace.

**For "deploy to cluster":** ask for kubeconfig context and namespace.

## Step 3: Run kagent Commands

```bash
# List all agents
kubectl get agents -n <namespace>

# Get agent details
kubectl describe agent <agent_name> -n <namespace>

# View agent logs (agent runs as a pod)
kubectl logs -l app=kagent,agent=<agent_name> -n <namespace> --tail=100

# Check agent status via CRD
kubectl get agent <agent_name> -n <namespace> -o jsonpath='{.status}'

# Run a task (patch the agent spec with a new task)
kubectl patch agent <agent_name> -n <namespace> \
  --type=merge -p '{"spec":{"task":"<task_description>"}}'
```

## Step 4: For "Create Agent" — Generate YAML Spec

Generate and display the Agent CRD manifest:

```yaml
apiVersion: kagent.dev/v1alpha1
kind: Agent
metadata:
  name: <agent_name>
  namespace: <namespace>
spec:
  modelConfig: <model_backend>-config
  tools:
    - name: kubectl
    - name: helm
  systemPrompt: |
    You are a Kubernetes operations agent. Use your tools to complete tasks
    safely and report findings clearly.
```

Then ask: "Apply this to the cluster? (yes / no)"

```bash
kubectl apply -f /tmp/<agent_name>-agent.yaml
```

## Step 5: For "Deploy Controller" — Helm Install

```bash
helm repo add kagent https://kagent-dev.github.io/kagent
helm repo update
helm install kagent kagent/kagent \
  -n kagent-system --create-namespace \
  --set model.backend=<model_backend>
```

Verify: `kubectl get pods -n kagent-system`
