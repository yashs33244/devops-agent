#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Adding Helm repositories..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
helm repo add grafana https://grafana.github.io/helm-charts 2>/dev/null || true
helm repo update

echo "Installing kube-prometheus-stack..."
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace \
  -f "$SCRIPT_DIR/values-kube-prometheus-stack.yaml" \
  --wait

echo "Installing Loki..."
helm upgrade --install loki grafana/loki \
  -n monitoring \
  -f "$SCRIPT_DIR/values-loki.yaml" \
  --wait

echo "Installing Promtail..."
helm upgrade --install promtail grafana/promtail \
  -n monitoring \
  -f "$SCRIPT_DIR/values-promtail.yaml" \
  --wait

echo "Applying ServiceMonitor and PodMonitor..."
kubectl apply -f "$SCRIPT_DIR/servicemonitor-api.yaml"
kubectl apply -f "$SCRIPT_DIR/podmonitor-chicklets.yaml"

echo "Applying Grafana dashboards..."
kubectl apply -f "$SCRIPT_DIR/dashboard-cluster.yaml"
kubectl apply -f "$SCRIPT_DIR/dashboard-api.yaml"
kubectl apply -f "$SCRIPT_DIR/dashboard-logs.yaml"

echo "Done. Grafana is available via: kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80"
