# Policy attached to the `admin` identity group. Members of this group
# (the chart-seeded admin user + anyone an existing admin promotes via
# group membership) can manage userpass credentials, identity entities,
# entity-aliases, and group membership — everything needed to onboard
# new users without resorting to `kubectl exec` against OpenBao.
#
# Granted via openbao.oidc-bootstrap.sh's ensure_group("admin",
# "nightshift-admin"). The policy is intentionally narrower than
# OpenBao's built-in `default` — it does NOT grant token-creation,
# system-mount-management, or KV access. Admins manage *people*; the
# nightshift-api SA's policy still owns secret-path access.

# --- userpass: create / list / delete users ---
path "auth/userpass/users" {
  capabilities = ["list"]
}
path "auth/userpass/users/*" {
  capabilities = ["create", "read", "update", "delete"]
}

# --- identity entities ---
path "identity/entity" {
  capabilities = ["create", "update"]
}
path "identity/entity/id" {
  capabilities = ["list"]
}
path "identity/entity/id/*" {
  capabilities = ["read", "update", "delete"]
}
path "identity/entity/name" {
  capabilities = ["list"]
}
path "identity/entity/name/*" {
  capabilities = ["read", "update", "delete"]
}

# --- identity entity-aliases (binds userpass credentials to entities) ---
path "identity/entity-alias" {
  capabilities = ["create", "update"]
}
path "identity/entity-alias/id" {
  capabilities = ["list"]
}
path "identity/entity-alias/id/*" {
  capabilities = ["read", "update", "delete"]
}

# --- identity groups: read membership + add/remove members ---
path "identity/group" {
  capabilities = ["create", "update"]
}
path "identity/group/id" {
  capabilities = ["list"]
}
path "identity/group/id/*" {
  capabilities = ["read", "update"]
}
path "identity/group/name" {
  capabilities = ["list"]
}
path "identity/group/name/*" {
  capabilities = ["read", "update"]
}

# --- enumerate auth methods to find the userpass mount accessor ---
path "sys/auth" {
  capabilities = ["read", "list"]
}
