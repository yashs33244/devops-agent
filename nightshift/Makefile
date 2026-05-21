BINARY_DIR := bin
GOFLAGS := -trimpath

# Dev image tags. Match the chart's default image.registry so kind-loaded
# images resolve without --set overrides.
REGISTRY ?= ghcr.io/nightshiftco
TAG      ?= latest

# kind cluster name. Override on the command line for parallel clusters.
KIND_CLUSTER ?= nightshift-dev

# Versions for the runtime-security stack. Pinned so verification is
# reproducible. See `make kind-runtime-security`.
CILIUM_VERSION   ?= 1.19.2
TETRAGON_VERSION ?= 1.2.0

# EKS quickstart targets — bundled in-cluster nightshift on EKS.
#
# Two ways to run eks-quickstart:
#   1. Provision a fresh cluster with terraform (`make eks-cluster-up`),
#      which writes deploy/eks/terraform/.tf-outputs.json. The EKS_*
#      vars below resolve from that file.
#   2. BYO existing cluster: export each EKS_* var (or pass on the
#      command line) before running eks-quickstart. Env / command-line
#      values always override the terraform fallback. Required for
#      eks-quickstart: EKS_CLUSTER_ARN, EKS_REGION, EKS_REGISTRY,
#      EKS_UI_HOST. Required additionally for eks-addons-up:
#      EKS_LETSENCRYPT_EMAIL.
TF_DIR := deploy/eks/terraform
TF_OUT := $(TF_DIR)/.tf-outputs.json

# `?=` so env vars and `make EKS_FOO=bar` win over the terraform fallback.
# The right-hand-side jq is recursive — re-runs on each reference, returns
# "" if the file is missing. eks-vars-check catches the resulting empties.
EKS_CLUSTER_ARN       ?= $(shell jq -r '.cluster_arn.value'       $(TF_OUT) 2>/dev/null)
EKS_REGION            ?= $(shell jq -r '.region.value'            $(TF_OUT) 2>/dev/null)
EKS_REGISTRY          ?= $(shell jq -r '.ecr_registry.value'      $(TF_OUT) 2>/dev/null)
EKS_UI_HOST           ?= $(shell jq -r '.ui_host.value'           $(TF_OUT) 2>/dev/null)
EKS_LETSENCRYPT_EMAIL ?= $(shell jq -r '.letsencrypt_email.value' $(TF_OUT) 2>/dev/null)

EKS_REGISTRY_NS ?= nightshift
EKS_TAG         ?= sha-$(shell git rev-parse --short HEAD)
EKS_NAMESPACE   ?= nightshift
EKS_RELEASE     ?= nightshift

# Pinned addon versions for `make eks-addons-up`.
INGRESS_NGINX_VERSION ?= 4.11.3
CERT_MANAGER_VERSION  ?= 1.16.2

.PHONY: all build test test-integration vet lint proto proto-lint proto-breaking docker docker-claude docker-ui helm-lint ci kind-up kind-down kind-load kind-deploy kind-deploy-with-openbao kind-deploy-with-ui kind-quickstart kind-quickstart-sqlite kind-uninstall kind-runtime-security eks-cluster-up eks-cluster-down eks-kubeconfig eks-addons-up eks-addons-down eks-runtime-security eks-quickstart eks-uninstall eks-context-check eks-vars-check eks-tf-outputs-check eks-ecr-login eks-ecr-push clean

all: build

build:
	go build $(GOFLAGS) -o $(BINARY_DIR)/nightshift-api ./cmd/nightshift-api
	go build $(GOFLAGS) -o $(BINARY_DIR)/nightshift-worker ./cmd/nightshift-worker

test:
	go test -race ./...

test-integration:
	go test -race -tags=integration ./test/integration/...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

proto:
	buf generate

proto-lint:
	buf lint

proto-breaking:
	buf breaking --against '.git#branch=master'

# Make the Nightshift API, Claude Woker, and UI images 
docker: docker-claude docker-ui
	docker build -f cmd/nightshift-api/Dockerfile    -t $(REGISTRY)/nightshift-api:$(TAG)    .
	docker build -f cmd/nightshift-worker/Dockerfile -t $(REGISTRY)/nightshift-worker:$(TAG) .

# Make the Claude worker image
docker-claude:
	docker build -f cmd/nightshift-worker-claude/Dockerfile \
	  -t $(REGISTRY)/nightshift-worker-claude:$(TAG) \
	  cmd/nightshift-worker-claude

# Make the Nightshift UI image 
docker-ui:
	docker build -f cmd/nightshift-ui/Dockerfile \
	  -t $(REGISTRY)/nightshift-ui:$(TAG) \
	  cmd/nightshift-ui

helm-lint:
	helm lint deploy/charts/nightshift \
	  --set tenant.name=dev \
	  --set nightshift_api.workerHmac=0123456789abcdef0123456789abcdef \
	  --set nightshift_api.objectsHmac=fedcba9876543210fedcba9876543210

# Runs CI 
ci: vet lint test proto-lint proto
	@if ! git diff --exit-code -- gen/; then \
	  echo "gen/ out of date — commit the result of 'make proto'"; \
	  exit 1; \
	fi

# Creates a kind cluster
kind-up:
	kind create cluster --name $(KIND_CLUSTER) --config deploy/kind/cluster.yaml

# Tears down a kind cluster
kind-down:
	kind delete cluster --name $(KIND_CLUSTER)

kind-load: docker
	kind load docker-image $(REGISTRY)/nightshift-api:$(TAG)            --name $(KIND_CLUSTER)
	kind load docker-image $(REGISTRY)/nightshift-worker:$(TAG)         --name $(KIND_CLUSTER)
	kind load docker-image $(REGISTRY)/nightshift-worker-claude:$(TAG)  --name $(KIND_CLUSTER)
	kind load docker-image $(REGISTRY)/nightshift-ui:$(TAG)             --name $(KIND_CLUSTER)

# Generate dev HMACs + MinIO password if missing, load images, helm
# install/upgrade. Drops the rendered chart into the `nightshift` namespace under the release name `nightshift`. 
kind-deploy: kind-load
	@mkdir -p deploy/kind
	@test -s deploy/kind/.dev-worker-hmac  || openssl rand -hex 32 > deploy/kind/.dev-worker-hmac
	@test -s deploy/kind/.dev-objects-hmac || openssl rand -hex 32 > deploy/kind/.dev-objects-hmac
	@test -s deploy/kind/.dev-minio-pass   || openssl rand -base64 24 > deploy/kind/.dev-minio-pass
	helm upgrade --install nightshift deploy/charts/nightshift \
	  --namespace nightshift --create-namespace \
	  --set tenant.name=dev \
	  --set image.registry=$(REGISTRY) \
	  --set image.tag=$(TAG) \
	  --set-file nightshift_api.workerHmac=deploy/kind/.dev-worker-hmac \
	  --set-file nightshift_api.objectsHmac=deploy/kind/.dev-objects-hmac \
	  --set nightshift_api.objectsBackend=s3 \
	  --set minio.enabled=true \
	  --set-file minio.rootPassword=deploy/kind/.dev-minio-pass \
	  --set 'nightshift_api.auth.staticTokens.cli=local-dev-token'

kind-deploy-with-openbao: kind-load
	@mkdir -p deploy/kind
	@test -s deploy/kind/.dev-worker-hmac  || openssl rand -hex 32 > deploy/kind/.dev-worker-hmac
	@test -s deploy/kind/.dev-objects-hmac || openssl rand -hex 32 > deploy/kind/.dev-objects-hmac
	@test -s deploy/kind/.dev-minio-pass   || openssl rand -base64 24 > deploy/kind/.dev-minio-pass
	helm upgrade --install nightshift deploy/charts/nightshift \
	  --namespace nightshift --create-namespace \
	  --set tenant.name=dev \
	  --set image.registry=$(REGISTRY) \
	  --set image.tag=$(TAG) \
	  --set-file nightshift_api.workerHmac=deploy/kind/.dev-worker-hmac \
	  --set-file nightshift_api.objectsHmac=deploy/kind/.dev-objects-hmac \
	  --set nightshift_api.objectsBackend=s3 \
	  --set minio.enabled=true \
	  --set-file minio.rootPassword=deploy/kind/.dev-minio-pass \
	  --set openbao.enabled=true \
	  --set 'nightshift_api.auth.staticTokens.cli=local-dev-token' \
	  --set 'nightshift_api.auth.adminTokens={cli}'

# kind-quickstart — Postgres-backed nightshift-api as a multi-replica
# Deployment. The chart's pre-install Migration Job applies schema
# before the Deployment rolls out. The UI is still a single-replica
# StatefulSet on SQLite (PR2 will migrate it to Postgres).
kind-quickstart: kind-load kind-runtime-security
	@mkdir -p deploy/kind
	@test -s deploy/kind/.dev-worker-hmac    || openssl rand -hex 32 > deploy/kind/.dev-worker-hmac
	@test -s deploy/kind/.dev-objects-hmac   || openssl rand -hex 32 > deploy/kind/.dev-objects-hmac
	@test -s deploy/kind/.dev-minio-pass     || openssl rand -base64 24 > deploy/kind/.dev-minio-pass
	@test -s deploy/kind/.dev-postgres-pass  || openssl rand -hex 24 > deploy/kind/.dev-postgres-pass
	helm upgrade --install nightshift deploy/charts/nightshift \
	  --namespace nightshift --create-namespace \
	  --set tenant.name=dev \
	  --set image.registry=$(REGISTRY) \
	  --set image.tag=$(TAG) \
	  --set-file nightshift_api.workerHmac=deploy/kind/.dev-worker-hmac \
	  --set-file nightshift_api.objectsHmac=deploy/kind/.dev-objects-hmac \
	  --set nightshift_api.recordsBackend=postgres \
	  --set nightshift_api.replicas=2 \
	  --set nightshift_api.objectsBackend=s3 \
	  --set nightshift_api.worker.repository=nightshift-worker-claude \
	  --set nightshift_api.workerClaude.enabled=true \
	  --set nightshift_api.sessionState.backend=object \
	  --set nightshift_api.sessionState.object.bucket=nightshift \
	  --set postgres.enabled=true \
	  --set-file postgres.password=deploy/kind/.dev-postgres-pass \
	  --set minio.enabled=true \
	  --set minio.console.enabled=true \
	  --set-file minio.rootPassword=deploy/kind/.dev-minio-pass \
	  --set openbao.enabled=true \
	  --set openbao.oidc.publicEndpoint=http://localhost:18200 \
	  --set ui.enabled=true \
	  --set ui.replicas=2 \
	  --set ui.publicURL=http://localhost:13000 \
	  --set 'ui.trustedOrigins={http://localhost:13000}' \
	  --set 'nightshift_api.auth.staticTokens.cli=local-dev-token' \
	  --set 'nightshift_api.auth.adminTokens={cli}'

# kind-quickstart-sqlite — original SQLite-backed StatefulSet path.
# Single replica per pod, per-pod PVC. Useful for fully-in-cluster
# dev or for environments where bringing up a Postgres pod is
# unwanted overhead. UI remains a single-replica StatefulSet.
kind-quickstart-sqlite: kind-load kind-runtime-security
	@mkdir -p deploy/kind
	@test -s deploy/kind/.dev-worker-hmac  || openssl rand -hex 32 > deploy/kind/.dev-worker-hmac
	@test -s deploy/kind/.dev-objects-hmac || openssl rand -hex 32 > deploy/kind/.dev-objects-hmac
	@test -s deploy/kind/.dev-minio-pass   || openssl rand -base64 24 > deploy/kind/.dev-minio-pass
	helm upgrade --install nightshift deploy/charts/nightshift \
	  --namespace nightshift --create-namespace \
	  --set tenant.name=dev \
	  --set image.registry=$(REGISTRY) \
	  --set image.tag=$(TAG) \
	  --set-file nightshift_api.workerHmac=deploy/kind/.dev-worker-hmac \
	  --set-file nightshift_api.objectsHmac=deploy/kind/.dev-objects-hmac \
	  --set nightshift_api.objectsBackend=s3 \
	  --set nightshift_api.worker.repository=nightshift-worker-claude \
	  --set nightshift_api.workerClaude.enabled=true \
	  --set nightshift_api.sessionState.backend=object \
	  --set nightshift_api.sessionState.object.bucket=nightshift \
	  --set minio.enabled=true \
	  --set minio.console.enabled=true \
	  --set-file minio.rootPassword=deploy/kind/.dev-minio-pass \
	  --set openbao.enabled=true \
	  --set openbao.oidc.publicEndpoint=http://localhost:18200 \
	  --set ui.enabled=true \
	  --set ui.publicURL=http://localhost:13000 \
	  --set 'ui.trustedOrigins={http://localhost:13000}' \
	  --set 'nightshift_api.auth.staticTokens.cli=local-dev-token' \
	  --set 'nightshift_api.auth.adminTokens={cli}'

kind-uninstall:
	helm uninstall nightshift --namespace nightshift || true
	kubectl delete namespace nightshift --ignore-not-found

kind-runtime-security:
	helm repo add cilium https://helm.cilium.io/ >/dev/null 2>&1 || true
	helm repo update cilium >/dev/null
	helm upgrade --install cilium cilium/cilium --version $(CILIUM_VERSION) \
	  --namespace kube-system \
	  --set image.pullPolicy=IfNotPresent \
	  --set ipam.mode=kubernetes \
	  --set hubble.enabled=true \
	  --set hubble.relay.enabled=true \
	  --set hubble.ui.enabled=true \
	  --wait --timeout 5m
	kubectl -n kube-system rollout status ds/cilium --timeout=2m
	helm upgrade --install tetragon cilium/tetragon --version $(TETRAGON_VERSION) \
	  --namespace kube-system \
	  --set tetragon.hostProcPath=/procHost \
	  --wait --timeout 5m
	kubectl -n kube-system rollout status ds/tetragon --timeout=2m
	@echo
	@echo "Cilium $(CILIUM_VERSION) + Hubble + Tetragon $(TETRAGON_VERSION) installed."
	@echo "Tail Tetragon events:"
	@echo "  kubectl logs -n kube-system -l app.kubernetes.io/name=tetragon -c export-stdout -f"
	@echo "Observe Hubble flows on a worker pod:"
	@echo "  kubectl exec -n kube-system ds/cilium -- hubble observe --pod ns-run-*"

# eks-vars-check — fail with a useful message when any EKS_* the
# downstream targets need is empty. Sources accepted (in priority order):
# command-line override, environment, $(TF_OUT) (terraform fallback).
eks-vars-check:
	@MISSING=""; \
	[ -n "$(EKS_CLUSTER_ARN)" ]       || MISSING="$$MISSING EKS_CLUSTER_ARN"; \
	[ -n "$(EKS_REGION)" ]            || MISSING="$$MISSING EKS_REGION"; \
	[ -n "$(EKS_REGISTRY)" ]          || MISSING="$$MISSING EKS_REGISTRY"; \
	[ -n "$(EKS_UI_HOST)" ]           || MISSING="$$MISSING EKS_UI_HOST"; \
	if [ -n "$$MISSING" ]; then \
	  echo "missing required EKS_* values:$$MISSING"; \
	  echo; \
	  echo "two ways to populate them:"; \
	  echo "  1) terraform path:   make eks-cluster-up"; \
	  echo "  2) BYO cluster:      export each missing variable, e.g."; \
	  echo "       export EKS_CLUSTER_ARN=arn:aws:eks:us-east-1:123:cluster/foo"; \
	  echo "       export EKS_REGION=us-east-1"; \
	  echo "       export EKS_REGISTRY=123.dkr.ecr.us-east-1.amazonaws.com"; \
	  echo "       export EKS_UI_HOST=nightshift.example.com"; \
	  exit 1; \
	fi

# eks-tf-outputs-check — narrower gate used only by targets that depend
# on terraform-managed state itself (eks-kubeconfig). BYO-cluster users
# never hit this; they wire their own kubeconfig out-of-band.
eks-tf-outputs-check:
	@test -s $(TF_OUT) || { \
	  echo "missing $(TF_OUT)."; \
	  echo "this target requires a terraform-provisioned cluster."; \
	  echo "either run 'make eks-cluster-up', or populate kubeconfig"; \
	  echo "yourself (e.g. 'aws eks update-kubeconfig ...')."; \
	  exit 1; \
	}

# eks-context-check — refuse to do anything when kubectl isn't pointed
# at the expected EKS cluster.
eks-context-check: eks-vars-check
	@CTX=$$(kubectl config current-context); \
	if [ "$$CTX" != "$(EKS_CLUSTER_ARN)" ]; then \
	  echo "current kube-context is $$CTX, expected $(EKS_CLUSTER_ARN)"; \
	  echo "switch with: kubectl config use-context $(EKS_CLUSTER_ARN)"; \
	  exit 1; \
	fi

# eks-cluster-up — provision VPC + EKS + node group + EBS CSI + gp3 SC
# + ECR repos. ~15 min on first apply. Caches outputs to .tf-outputs.json
# for the rest of the EKS targets to read.
eks-cluster-up:
	cd $(TF_DIR) && terraform init -upgrade
	cd $(TF_DIR) && terraform apply
	cd $(TF_DIR) && terraform output -json > $(notdir $(TF_OUT))
	@echo
	@echo "Cluster up. Populate kubeconfig:"
	@echo "  eval \$$(make eks-kubeconfig)"
	@echo "Then: make eks-addons-up"

# eks-cluster-down — teardown. Will fail if there are LoadBalancer
# Services or PVCs that still own AWS resources; run eks-uninstall +
# eks-addons-down first.
eks-cluster-down:
	cd $(TF_DIR) && terraform destroy
	rm -f $(TF_OUT)

# eks-kubeconfig — print the `aws eks update-kubeconfig` command. Use
# with `eval $(make eks-kubeconfig)` to populate ~/.kube/config.
eks-kubeconfig: eks-tf-outputs-check
	@cd $(TF_DIR) && terraform output -raw kubeconfig_cmd

# eks-addons-up — install ingress-nginx + cert-manager and create the
# letsencrypt-prod ClusterIssuer the chart's UI ingress references.
# After this, point a CNAME from $(EKS_UI_HOST) at the printed ELB
# hostname before running `make eks-quickstart` — cert-manager's
# HTTP-01 challenge needs DNS to be live to issue the cert.
eks-addons-up: eks-context-check
	@test -n "$(EKS_LETSENCRYPT_EMAIL)" || { \
	  echo "EKS_LETSENCRYPT_EMAIL is empty (registered with the cert-manager"; \
	  echo "ClusterIssuer for ACME expiry warnings). Set it via:"; \
	  echo "  export EKS_LETSENCRYPT_EMAIL=you@example.com"; \
	  echo "or run 'make eks-cluster-up' which writes it to $(TF_OUT)."; \
	  exit 1; \
	}
	helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx >/dev/null 2>&1 || true
	helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true
	helm repo update ingress-nginx jetstack >/dev/null
	helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
	  --version $(INGRESS_NGINX_VERSION) \
	  --namespace ingress-nginx --create-namespace \
	  --set controller.service.type=LoadBalancer \
	  --wait --timeout 10m
	helm upgrade --install cert-manager jetstack/cert-manager \
	  --version v$(CERT_MANAGER_VERSION) \
	  --namespace cert-manager --create-namespace \
	  --set installCRDs=true \
	  --wait --timeout 5m
	@printf '%s\n' \
	  'apiVersion: cert-manager.io/v1' \
	  'kind: ClusterIssuer' \
	  'metadata:' \
	  '  name: letsencrypt-prod' \
	  'spec:' \
	  '  acme:' \
	  '    server: https://acme-v02.api.letsencrypt.org/directory' \
	  '    email: $(EKS_LETSENCRYPT_EMAIL)' \
	  '    privateKeySecretRef:' \
	  '      name: letsencrypt-prod' \
	  '    solvers:' \
	  '      - http01:' \
	  '          ingress:' \
	  '            class: nginx' \
	  | kubectl apply -f -
	@echo
	@ELB=$$(kubectl -n ingress-nginx get svc ingress-nginx-controller \
	  -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'); \
	echo "ingress-nginx ELB is up:"; \
	echo "  CNAME $(EKS_UI_HOST) -> $$ELB"; \
	echo; \
	echo "create the CNAME at your DNS provider, wait for it to propagate,"; \
	echo "then run: make eks-quickstart"

# eks-addons-down — reverse eks-addons-up. Order matters: ClusterIssuer
# first, then cert-manager, then ingress-nginx (which owns the ELB).
eks-addons-down: eks-context-check
	-kubectl delete clusterissuer letsencrypt-prod
	-helm uninstall cert-manager -n cert-manager
	-kubectl delete namespace cert-manager --ignore-not-found
	-helm uninstall ingress-nginx -n ingress-nginx
	-kubectl delete namespace ingress-nginx --ignore-not-found

# eks-runtime-security — optional opt-in. Mirrors kind-runtime-security
# but installs Cilium in chaining mode on top of the AWS VPC CNI (the
# stable Cilium docs' EKS path; full vpc-cni replacement is no longer
# the documented default). Tetragon runs with its native /proc, no kind
# /procHost override needed. Prereq: AWS VPC CNI ≥ 1.11.2 (the EKS
# managed addon installed by terraform is well past that).
#
# `kubectl rollout restart ds/aws-node` is included so existing
# vpc-cni pods pick up the chaining config — needed only for re-runs
# on a populated cluster (on a fresh quickstart no app pods exist yet).
eks-runtime-security: eks-context-check
	helm repo add cilium https://helm.cilium.io/ >/dev/null 2>&1 || true
	helm repo update cilium >/dev/null
	helm upgrade --install cilium cilium/cilium --version $(CILIUM_VERSION) \
	  --namespace kube-system \
	  --set cni.chainingMode=aws-cni \
	  --set cni.exclusive=false \
	  --set enableIPv4Masquerade=false \
	  --set routingMode=native \
	  --set hubble.enabled=true \
	  --set hubble.relay.enabled=true \
	  --set hubble.ui.enabled=true \
	  --wait --timeout 10m
	kubectl -n kube-system rollout restart ds/aws-node
	kubectl -n kube-system rollout status ds/aws-node --timeout=5m
	kubectl -n kube-system rollout status ds/cilium --timeout=5m
	helm upgrade --install tetragon cilium/tetragon --version $(TETRAGON_VERSION) \
	  --namespace kube-system \
	  --wait --timeout 5m
	kubectl -n kube-system rollout status ds/tetragon --timeout=2m
	@echo
	@echo "Cilium $(CILIUM_VERSION) (aws-cni chaining) + Hubble + Tetragon $(TETRAGON_VERSION) installed."
	@echo "Tail Tetragon events:"
	@echo "  kubectl logs -n kube-system -l app.kubernetes.io/name=tetragon -c export-stdout -f"
	@echo "Observe Hubble flows on a worker pod:"
	@echo "  kubectl exec -n kube-system ds/cilium -- hubble observe --pod ns-run-*"

# eks-ecr-login — docker login against the ECR registry. Inherits the
# operator's existing AWS env.
eks-ecr-login: eks-vars-check
	aws ecr get-login-password --region $(EKS_REGION) | \
	  docker login --username AWS --password-stdin $(EKS_REGISTRY)

# eks-ecr-push — build images with $(EKS_TAG) and push. The terraform
# path manages the repos; BYO-cluster users must pre-create the four
# nightshift/<image> repos under $(EKS_REGISTRY). Uses `docker buildx
# --platform linux/amd64 --push` so the manifest only carries amd64
# (matching EKS nodes) regardless of host arch — Mac M-series would
# otherwise produce arm64-only manifests that fail to pull on
# linux/amd64 nodes.
eks-ecr-push: eks-vars-check
	docker buildx build --platform linux/amd64 --push -f cmd/nightshift-api/Dockerfile           -t $(EKS_REGISTRY)/$(EKS_REGISTRY_NS)/nightshift-api:$(EKS_TAG)           .
	docker buildx build --platform linux/amd64 --push -f cmd/nightshift-worker/Dockerfile        -t $(EKS_REGISTRY)/$(EKS_REGISTRY_NS)/nightshift-worker:$(EKS_TAG)        .
	docker buildx build --platform linux/amd64 --push -f cmd/nightshift-worker-claude/Dockerfile -t $(EKS_REGISTRY)/$(EKS_REGISTRY_NS)/nightshift-worker-claude:$(EKS_TAG) cmd/nightshift-worker-claude
	docker buildx build --platform linux/amd64 --push -f cmd/nightshift-ui/Dockerfile            -t $(EKS_REGISTRY)/$(EKS_REGISTRY_NS)/nightshift-ui:$(EKS_TAG)            cmd/nightshift-ui

# eks-quickstart — bundled in-cluster nightshift on EKS. Same shape as
# kind-quickstart, with cloud overrides: ECR images, gp3 PVCs, nginx
# Ingress + cert-manager TLS for the UI host.
eks-quickstart: eks-context-check eks-ecr-login eks-ecr-push
	@mkdir -p deploy/eks
	@test -s deploy/eks/.eks-worker-hmac    || openssl rand -hex 32 > deploy/eks/.eks-worker-hmac
	@test -s deploy/eks/.eks-objects-hmac   || openssl rand -hex 32 > deploy/eks/.eks-objects-hmac
	@test -s deploy/eks/.eks-postgres-pass  || openssl rand -hex 24 > deploy/eks/.eks-postgres-pass
	@test -s deploy/eks/.eks-minio-pass     || openssl rand -base64 24 > deploy/eks/.eks-minio-pass
	helm upgrade --install $(EKS_RELEASE) deploy/charts/nightshift \
	  --namespace $(EKS_NAMESPACE) --create-namespace \
	  --timeout 15m \
	  --set tenant.name=eks \
	  --set image.registry=$(EKS_REGISTRY)/$(EKS_REGISTRY_NS) \
	  --set image.tag=$(EKS_TAG) \
	  --set-file nightshift_api.workerHmac=deploy/eks/.eks-worker-hmac \
	  --set-file nightshift_api.objectsHmac=deploy/eks/.eks-objects-hmac \
	  --set nightshift_api.recordsBackend=postgres \
	  --set nightshift_api.replicas=2 \
	  --set nightshift_api.objectsBackend=s3 \
	  --set nightshift_api.worker.repository=nightshift-worker-claude \
	  --set nightshift_api.workerClaude.enabled=true \
	  --set nightshift_api.sessionState.backend=object \
	  --set nightshift_api.sessionState.object.bucket=nightshift \
	  --set postgres.enabled=true \
	  --set postgres.storage.storageClassName=gp3 \
	  --set-file postgres.password=deploy/eks/.eks-postgres-pass \
	  --set minio.enabled=true \
	  --set minio.storage.storageClassName=gp3 \
	  --set-file minio.rootPassword=deploy/eks/.eks-minio-pass \
	  --set openbao.enabled=true \
	  --set openbao.storage.storageClassName=gp3 \
	  --set ui.enabled=true \
	  --set ui.replicas=2 \
	  --set ui.publicURL=https://$(EKS_UI_HOST) \
	  --set 'ui.trustedOrigins={https://$(EKS_UI_HOST)}' \
	  --set ui.ingress.enabled=true \
	  --set ui.ingress.className=nginx \
	  --set ui.ingress.host=$(EKS_UI_HOST) \
	  --set ui.ingress.clusterIssuer=letsencrypt-prod

# eks-uninstall — reverse eks-quickstart. Refuses to run if the
# kube-context isn't the expected EKS cluster. Leaves the cluster +
# addons in place; pair with eks-addons-down + eks-cluster-down for a
# full teardown.
eks-uninstall: eks-context-check
	helm uninstall $(EKS_RELEASE) --namespace $(EKS_NAMESPACE) || true
	kubectl delete namespace $(EKS_NAMESPACE) --ignore-not-found

clean:
	rm -rf $(BINARY_DIR) gen/go/nightshift gen/openapiv2/nightshift
