# nightshift Helm chart

## Deploy to kind 

End-to-end happy path on a fresh kind cluster, using the `make` targets at the repo root. There are three install tiers. 

| Target | Brings up | Use for |
|---|---|---|
| `make kind-deploy` | API + worker + MinIO | Smoke-test the gRPC/REST surface |
| `make kind-deploy-with-openbao` | + OpenBao OIDC + admin user | Exercise OIDC bearer auth, secret backend |
| `make kind-deploy-with-ui` | + Next.js UI (chunk 19) | **First-touch** — full stack, browser-driven |

Bring up the cluster if you haven't already:

```bash
make kind-up
```

Deploy the full stack including UI;
```bash
make kind-deploy-with-ui
```

Or, for API only:
```bash
make kind-deploy                 # API + worker + MinIO only
make kind-deploy-with-openbao    # + OpenBao OIDC
```

