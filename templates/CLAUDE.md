# templates/

Jinja2-renderable templates consumed by the tools in `tools/`. Never edit generated output in `workspace/` — edit the templates here instead.

## What's Here

| Directory | Contents |
|-----------|---------|
| `terraform/` | Terraform modules for AWS (EKS+ECR+RDS+VPC), Azure (AKS+ACR+PostgreSQL), GCP (GKE+Artifact Registry+Cloud SQL) |
| `dockerfiles/` | Multi-stage distroless Dockerfiles for Node.js, Python, Go, Java, Rust |
| `github-actions/` | CI pipeline, CD pipeline, security scan, and reusable workflow components |
| `helm/` | Production Helm chart with security contexts, resource limits, probes, KEDA support |
| `keda/` | HTTPScaledObject for car-painter scale-to-zero pattern |
| `monitoring/` | Prometheus + Grafana Docker Compose stack with pre-built dashboards |

## Key Details

- Template variables use `{{VARNAME}}` syntax (double-braces, uppercase)
- Tools call `tools/terraform_gen.py`, `tools/helm_gen.py`, etc. to render these templates — do not render manually
- After rendering Terraform, always run `terraform fmt && terraform validate -backend=false`
- Dockerfiles use pinned `@sha256:` digests — see `dockerfiles/README.md` for how to obtain real digests

## Related

- `tools/` — tools that render these templates into `workspace/<service>/`
- Root `CLAUDE.md` — lists all template paths and their purposes
