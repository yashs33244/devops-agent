# templates/monitoring/

Prometheus + Grafana Docker Compose stack for local observability of the sample services.

## What's Here

```
docker-compose.monitoring.yml   # Prometheus + Grafana + Alertmanager
prometheus.yml                  # Prometheus scrape config (targets: python-api:8000, go-api:8080)
alertmanager.yml                # Alertmanager routing rules
kube-prometheus-stack.yaml      # Helm values for kube-prometheus-stack (cluster deployment)
grafana/
  provisioning/
    dashboards/                 # Auto-provisioned Grafana dashboard JSON files
    datasources/                # Auto-provisioned Prometheus datasource config
```

## How to Use / Run

```bash
# Start local monitoring stack
docker compose -f templates/monitoring/docker-compose.monitoring.yml up -d

# Access Grafana
open http://localhost:3001
# Default credentials: admin / devops-agent

# Access Prometheus
open http://localhost:9090

# Access Alertmanager
open http://localhost:9093

# Stop stack
docker compose -f templates/monitoring/docker-compose.monitoring.yml down

# Deploy to Kubernetes (kube-prometheus-stack)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  -f templates/monitoring/kube-prometheus-stack.yaml \
  -n monitoring --create-namespace
```

## Key Details

- **Prometheus**: `prom/prometheus:v3.0.1` on port 9090
- **Grafana**: `grafana/grafana:11.3.1` on port 3001 (mapped from container 3000)
- **Grafana password**: `devops-agent` (set via `GF_SECURITY_ADMIN_PASSWORD`)
- Prometheus is pre-configured to scrape `services/python-api` (`:8000/metrics`) and `services/go-api` (`:8080/metrics`)
- Grafana dashboards are auto-provisioned — no manual import needed
- `kube-prometheus-stack.yaml` is for cluster deployment (not the local Docker Compose stack)
- Prometheus storage: named Docker volume `prometheus_data` — persists across restarts

## Related

- `services/python-api/` and `services/go-api/` — expose `/metrics` endpoints scraped by Prometheus
- `templates/helm/` — Helm chart includes `serviceMonitor` support for cluster-level Prometheus scraping
- `tools/local_test.py` — starts emulators; monitoring stack is separate and must be started manually
