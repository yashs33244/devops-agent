# agents/plural/

Elixir/Phoenix GitOps multi-cloud deployment platform — provides a control plane for Helm, ArgoCD, and multi-tenant cluster management.

## What's Here

```
apps/
  api/        # GraphQL + REST API (Phoenix)
  core/       # Business logic, cluster sync, Helm operations
  cron/       # Scheduled jobs (cleanup, reconciliation)
  email/      # Transactional email
  graphql/    # Absinthe GraphQL schema
  rtc/        # Real-time channels (Phoenix Channels)
  worker/     # Background job processing
config/       # Per-environment Elixir config
schema/       # GraphQL schema SDL
www/          # Static assets / docs site
rel/          # Release configuration (Distillery/Mix Release)
Dockerfile    # Production image
docker-compose.yml       # Full local stack
docker-compose.test.yml  # Test stack
mix.exs / mix.lock       # Umbrella project definition
```

## How to Use / Run

```bash
# Install Elixir deps
mix deps.get

# Run tests
mix test

# Start local dev stack (Postgres + app)
docker compose up -d
mix phx.server

# Build production release
docker build -t plural:local .
```

## Key Details

- **Language**: Elixir (Mix umbrella project)
- **Framework**: Phoenix + Absinthe (GraphQL)
- **Version**: derived from `git describe --dirty=+dirty` via `mix.exs`
- `docker-compose.yml` is the canonical local dev environment
- The GraphQL API is the primary integration surface; see `schema/` for the SDL
- Real-time cluster sync runs in the `core` app; `cron` handles reconciliation loops
- Firebase config (`firebase.json`) suggests FCM push notifications for mobile clients

## Related

- `tools/cicd_setup.py` — generates GitHub Actions CD pipelines that can target a Plural-managed cluster
- `agents/kagent/` — Kubernetes-native agent framework that can run on clusters managed by Plural
- `templates/terraform/` — provisions the underlying cloud infrastructure that Plural deploys onto
