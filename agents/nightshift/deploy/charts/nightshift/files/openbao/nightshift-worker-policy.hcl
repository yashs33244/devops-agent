# Policy attached to the `nightshift-worker` Kubernetes-auth role.
# Worker pods authenticate as this role to read the worker HMAC and
# (chunk 14, nightshift-worker-claude only) the Anthropic API key.
# Strict read-only and scoped — workers must never have KV write
# access, and must never see static tokens or per-user connector
# tokens.

path "secret/data/nightshift/worker-hmac" {
  capabilities = ["read"]
}

# Used by the chunk-14 nightshift-worker-claude image. Operators must
# pre-seed the value (e.g. `bao kv put secret/nightshift/anthropic-api-key
# api-key=sk-ant-...`); read returns 404 until then. Granting
# unconditionally keeps the policy static — operators who don't run
# the LLM worker simply leave the path unseeded.
path "secret/data/nightshift/anthropic-api-key" {
  capabilities = ["read"]
}
