# Policy attached to the `nightshift-api` Kubernetes-auth role. The
# nightshift-api pod authenticates via its ServiceAccount token; the
# OpenBao secrets.Secrets backend impl consumes this policy.
#
# Grants:
#   - identity directory reads (share/permissions UIs)
#   - KV reads on shared nightshift platform secrets
#   - per-user KV read/write/delete on connector tokens
#     (nightshift/tokens/<user_id>/<connector>)
#   - KV read/write/delete on Native OAuth dispenser paths
#     (oauth-servers, oauth-pending, oauth-tokens)

# --- identity directory ---
path "identity/entity/name" {
  capabilities = ["list"]
}
path "identity/entity/name/*" {
  capabilities = ["read"]
}
path "identity/entity/id/*" {
  capabilities = ["read"]
}
# Chunk-19 user-discovery endpoint reads members of the configured
# user group (chart default "user") so the share-dialog dropdown can
# resolve entity IDs to display names. Read-only — this policy must
# never grant write on identity to the API.
path "identity/group/name/*" {
  capabilities = ["read"]
}

# --- shared nightshift platform secrets (read-only) ---
path "secret/data/nightshift/worker-hmac" {
  capabilities = ["read"]
}
path "secret/data/nightshift/static-tokens" {
  capabilities = ["read"]
}
path "secret/data/nightshift/connectors/*" {
  capabilities = ["read"]
}

# --- per-user connector tokens (KV v2: data + metadata for delete) ---
path "secret/data/nightshift/tokens/*" {
  capabilities = ["create", "read", "update", "delete"]
}
path "secret/metadata/nightshift/tokens/*" {
  capabilities = ["read", "delete", "list"]
}

# --- Native OAuth dispenser (KV v2: data + metadata for delete) ---
# The Native dispenser persists three classes of records under
# secret/nightshift/oauth-* via the secrets.Secrets interface:
#   - oauth-servers/<name>      → server config (client_id/secret/urls)
#   - oauth-pending/<state>     → PKCE code_verifier between authorize
#                                 and exchange (10-min TTL enforced
#                                 in-process by Native)
#   - oauth-tokens/<credName>   → per-user access_token + refresh_token
# All three need create/read/update/delete on data/, plus delete/list
# on metadata/ for KV-v2 cascade-delete from the connector teardown
# path.
path "secret/data/nightshift/oauth-servers/*" {
  capabilities = ["create", "read", "update", "delete"]
}
path "secret/metadata/nightshift/oauth-servers/*" {
  capabilities = ["read", "delete", "list"]
}
path "secret/data/nightshift/oauth-pending/*" {
  capabilities = ["create", "read", "update", "delete"]
}
path "secret/metadata/nightshift/oauth-pending/*" {
  capabilities = ["read", "delete", "list"]
}
path "secret/data/nightshift/oauth-tokens/*" {
  capabilities = ["create", "read", "update", "delete"]
}
path "secret/metadata/nightshift/oauth-tokens/*" {
  capabilities = ["read", "delete", "list"]
}
