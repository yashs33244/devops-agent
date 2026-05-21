#!/bin/sh
# Idempotent bootstrap for the nightshift OIDC identity provider in
# this tenant's OpenBao. Re-runs are safe: every step either checks
# for existing state or uses PUT.
#
# Required env (injected by the Helm Job spec):
#   BAO_ADDR                http://openbao.<ns>.svc:8200
#   BAO_TOKEN               root or equivalently-privileged token
#   OIDC_ISSUER             scheme+host the OIDC provider issues at
#   OIDC_CLIENT_NAME        human name of the OIDC client (e.g. nightshift-api)
#   CLIENT_REDIRECT_URI     primary browser/CLI callback
#   CLIENT_REDIRECT_URIS    optional comma-separated list (overrides CLIENT_REDIRECT_URI)
#   ADMIN_SEED_USER         initial admin username (optional)
#   ADMIN_SEED_PASSWORD     initial admin password (optional)
#   OIDC_CLIENT_SECRET_K8S  K8s Secret to publish client credentials into
#   K8S_NAMESPACE           release namespace
#   API_SA_NAME             nightshift-api ServiceAccount name (k8s-auth role binding)
#   WORKER_SA_NAME          nightshift-worker ServiceAccount name (k8s-auth role binding)

set -eu

log() { printf '[bootstrap] %s\n' "$*" >&2; }

CURL_OPTS='-sS --retry 5 --retry-connrefused --retry-delay 2 --retry-max-time 60 --connect-timeout 10'

api() {
  method="$1"; path="$2"; body="${3:-}"
  if [ -n "$body" ]; then
    curl $CURL_OPTS -X "$method" -H "X-Vault-Token: $BAO_TOKEN" -H 'Content-Type: application/json' --data @"$body" "$BAO_ADDR$path"
  else
    curl $CURL_OPTS -X "$method" -H "X-Vault-Token: $BAO_TOKEN" "$BAO_ADDR$path"
  fi
}

api_code() {
  method="$1"; path="$2"; body="${3:-}"
  if [ -n "$body" ]; then
    curl $CURL_OPTS -o /dev/null -w '%{http_code}' -X "$method" -H "X-Vault-Token: $BAO_TOKEN" -H 'Content-Type: application/json' --data @"$body" "$BAO_ADDR$path"
  else
    curl $CURL_OPTS -o /dev/null -w '%{http_code}' -X "$method" -H "X-Vault-Token: $BAO_TOKEN" "$BAO_ADDR$path"
  fi
}

json_str() {
  key="$1"
  grep -oE "\"$key\":\"[^\"]*\"" | head -1 | cut -d'"' -f4
}

data_id() {
  # Top-level "id" out of {"data":{...}}. The aliases array carries
  # its own ids; strip it (non-greedy, scalars only) so we don't
  # accidentally pick an alias id.
  sed 's/.*"data":{//' | sed 's/"aliases":\[[^]]*\]//' | grep -oE '"id":"[^"]*"' | head -1 | cut -d'"' -f4
}

wait_for_unseal() {
  log "waiting for openbao to be unsealed..."
  i=0
  while true; do
    state=$(curl -sS "$BAO_ADDR/v1/sys/seal-status" 2>/dev/null || true)
    if echo "$state" | grep -q '"sealed":false'; then
      log "openbao unsealed"
      return 0
    fi
    i=$((i + 1))
    if [ "$i" -gt 60 ]; then
      log "timed out waiting for unseal"
      return 1
    fi
    sleep 2
  done
}

enable_userpass() {
  code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
    -H "X-Vault-Token: $BAO_TOKEN" -H 'Content-Type: application/json' \
    --data '{"type":"userpass"}' \
    "$BAO_ADDR/v1/sys/auth/userpass")
  case "$code" in
    204|400) log "userpass enabled (code=$code)" ;;
    *) log "failed to enable userpass (code=$code)"; return 1 ;;
  esac
}

enable_kv_v2() {
  # secret/ holds shared nightshift platform secrets + per-user
  # connector tokens. Mounted now so the chunk-12 OpenBao Secrets
  # backend impl finds it pre-provisioned. Empty until then.
  code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
    -H "X-Vault-Token: $BAO_TOKEN" -H 'Content-Type: application/json' \
    --data '{"type":"kv","options":{"version":"2"}}' \
    "$BAO_ADDR/v1/sys/mounts/secret")
  case "$code" in
    204|400) log "kv v2 enabled at secret/ (code=$code)" ;;
    *) log "failed to enable kv v2 (code=$code)"; return 1 ;;
  esac
}

enable_kubernetes_auth() {
  code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
    -H "X-Vault-Token: $BAO_TOKEN" -H 'Content-Type: application/json' \
    --data '{"type":"kubernetes"}' \
    "$BAO_ADDR/v1/sys/auth/kubernetes")
  case "$code" in
    204|400) log "kubernetes auth enabled (code=$code)" ;;
    *) log "failed to enable kubernetes auth (code=$code)"; return 1 ;;
  esac
  # Pull the long-lived reviewer JWT + cluster CA from the
  # openbao-reviewer-token Secret. K8s populates these fields
  # asynchronously after the Secret is created; wait briefly.
  log "waiting for openbao-reviewer-token to populate..."
  i=0
  while true; do
    JWT_B64=$(kubectl -n "$K8S_NAMESPACE" get secret openbao-reviewer-token -o jsonpath='{.data.token}' 2>/dev/null || true)
    if [ -n "$JWT_B64" ]; then break; fi
    i=$((i + 1))
    if [ "$i" -gt 30 ]; then
      log "timed out waiting for openbao-reviewer-token Secret to populate"
      return 1
    fi
    sleep 1
  done
  CA_B64=$(kubectl -n "$K8S_NAMESPACE" get secret openbao-reviewer-token -o jsonpath='{.data.ca\.crt}')
  REVIEWER_JWT=$(printf '%s' "$JWT_B64" | base64 -d)
  CA_CERT=$(printf '%s' "$CA_B64" | base64 -d)
  jq -n --arg ca "$CA_CERT" --arg jwt "$REVIEWER_JWT" \
    --arg host "https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT" \
    '{kubernetes_host:$host, kubernetes_ca_cert:$ca, token_reviewer_jwt:$jwt, disable_iss_validation:true}' \
    > /tmp/k8s-config.json
  api POST "/v1/auth/kubernetes/config" /tmp/k8s-config.json >/dev/null
  log "kubernetes auth method configured with explicit ca + reviewer JWT"
}

ensure_group() {
  name="$1"
  policies="$2"  # optional comma-separated policy list
  code=$(api_code GET "/v1/identity/group/name/$name")
  if [ "$code" = "200" ]; then
    log "group $name already exists"
  else
    log "creating group $name"
    printf '{"type":"internal"}' > /tmp/group.json
    api PUT "/v1/identity/group/name/$name" /tmp/group.json >/dev/null
  fi
  if [ -n "$policies" ]; then
    log "ensuring group $name policies = $policies"
    json_list=$(printf '%s' "$policies" | awk -F, 'BEGIN{printf "["} {for(i=1;i<=NF;i++){if(i>1)printf ","; printf "\"%s\"",$i}} END{printf "]"}')
    printf '{"policies":%s}' "$json_list" > /tmp/group-policies.json
    api PUT "/v1/identity/group/name/$name" /tmp/group-policies.json >/dev/null
  fi
}

ensure_policy() {
  name="$1"; file="$2"
  if [ ! -r "$file" ]; then
    log "FATAL: policy file $file not readable"
    return 1
  fi
  log "writing policy $name"
  policy_b64=$(base64 -w0 < "$file")
  printf '{"policy":"%s"}' "$policy_b64" > /tmp/policy.json
  api PUT "/v1/sys/policies/acl/$name" /tmp/policy.json >/dev/null
}

ensure_k8s_role() {
  role="$1"; sa="$2"; policy="$3"
  log "creating K8s-auth role $role -> sa=$sa policy=$policy"
  printf '{"bound_service_account_names":["%s"],"bound_service_account_namespaces":["%s"],"token_policies":["%s"],"token_ttl":"1h","token_max_ttl":"24h"}' \
    "$sa" "$K8S_NAMESPACE" "$policy" > /tmp/role.json
  api PUT "/v1/auth/kubernetes/role/$role" /tmp/role.json >/dev/null
}

ensure_key() {
  code=$(api_code GET "/v1/identity/oidc/key/nightshift")
  if [ "$code" = "200" ]; then
    log "oidc key nightshift already exists"
    return 0
  fi
  log "creating oidc key nightshift"
  printf '{"rotation_period":"24h","verification_ttl":"24h","algorithm":"RS256","allowed_client_ids":["*"]}' > /tmp/key.json
  api PUT "/v1/identity/oidc/key/nightshift" /tmp/key.json >/dev/null
}

ensure_scope() {
  log "writing oidc scope groups"
  template='{"groups": {{identity.entity.groups.names}}, "email": {{identity.entity.metadata.email}}, "name": {{identity.entity.name}}}'
  template_b64=$(printf '%s' "$template" | base64 -w0)
  printf '{"description":"email, name, and identity group memberships","template":"%s"}' \
    "$template_b64" > /tmp/scope.json
  api PUT "/v1/identity/oidc/scope/groups" /tmp/scope.json >/dev/null
}

ensure_assignment() {
  log "writing oidc assignment nightshift"
  admin_gid=$(api GET "/v1/identity/group/name/admin" | data_id)
  user_gid=$(api GET "/v1/identity/group/name/user" | data_id)
  if [ -z "$admin_gid" ] || [ -z "$user_gid" ]; then
    log "FATAL: admin/user group ids unresolved"
    return 1
  fi
  printf '{"group_ids":["%s","%s"]}' "$admin_gid" "$user_gid" > /tmp/assign.json
  api PUT "/v1/identity/oidc/assignment/nightshift" /tmp/assign.json >/dev/null
}

ensure_client() {
  log "writing oidc client $OIDC_CLIENT_NAME"
  uris="${CLIENT_REDIRECT_URIS:-$CLIENT_REDIRECT_URI}"
  json_list=$(printf '%s' "$uris" | awk -F, 'BEGIN{printf "["} {for(i=1;i<=NF;i++){if(i>1)printf ","; printf "\"%s\"",$i}} END{printf "]"}')
  printf '{"redirect_uris":%s,"assignments":["nightshift"],"key":"nightshift","id_token_ttl":"24h","access_token_ttl":"24h","client_type":"confidential"}' \
    "$json_list" > /tmp/client.json
  api PUT "/v1/identity/oidc/client/$OIDC_CLIENT_NAME" /tmp/client.json >/dev/null
}

ensure_provider() {
  log "writing oidc provider nightshift"
  api_client_id=$(api GET "/v1/identity/oidc/client/$OIDC_CLIENT_NAME" | json_str client_id)
  if [ -z "$api_client_id" ]; then
    log "FATAL: cannot read client_id for $OIDC_CLIENT_NAME (run ensure_client first)"
    return 1
  fi
  ui_client_id=""
  if [ -n "${UI_CLIENT_NAME:-}" ] && [ -n "${UI_CLIENT_REDIRECT_URI:-}" ]; then
    ui_client_id=$(api GET "/v1/identity/oidc/client/$UI_CLIENT_NAME" | json_str client_id || echo "")
  fi
  if [ -n "$ui_client_id" ]; then
    allowed="[\"$api_client_id\",\"$ui_client_id\"]"
  else
    allowed="[\"$api_client_id\"]"
  fi
  printf '{"issuer":"%s","allowed_client_ids":%s,"scopes_supported":["groups"]}' \
    "$OIDC_ISSUER" "$allowed" > /tmp/provider.json
  api PUT "/v1/identity/oidc/provider/nightshift" /tmp/provider.json >/dev/null
}

ensure_ui_client() {
  log "writing oidc ui-client $UI_CLIENT_NAME"
  uris="$UI_CLIENT_REDIRECT_URI"
  json_list=$(printf '%s' "$uris" | awk -F, 'BEGIN{printf "["} {for(i=1;i<=NF;i++){if(i>1)printf ","; printf "\"%s\"",$i}} END{printf "]"}')
  printf '{"redirect_uris":%s,"assignments":["nightshift"],"key":"nightshift","id_token_ttl":"24h","access_token_ttl":"24h","client_type":"confidential"}' \
    "$json_list" > /tmp/ui-client.json
  api PUT "/v1/identity/oidc/client/$UI_CLIENT_NAME" /tmp/ui-client.json >/dev/null
}

publish_ui_client_secret_to_k8s() {
  log "publishing UI client credentials to k8s secret $UI_CLIENT_SECRET_K8S"
  body=$(api GET "/v1/identity/oidc/client/$UI_CLIENT_NAME")
  client_id=$(echo "$body" | json_str client_id)
  secret=$(echo "$body" | json_str client_secret)
  if [ -z "$client_id" ] || [ -z "$secret" ]; then
    log "FATAL: ui client credentials not available"
    return 1
  fi
  kubectl -n "$K8S_NAMESPACE" create secret generic "$UI_CLIENT_SECRET_K8S" \
    --from-literal=client_id="$client_id" \
    --from-literal=client_secret="$secret" \
    --dry-run=client -o yaml | kubectl apply -f -
}

publish_client_secret_to_k8s() {
  log "publishing client credentials to k8s secret $OIDC_CLIENT_SECRET_K8S"
  body=$(api GET "/v1/identity/oidc/client/$OIDC_CLIENT_NAME")
  client_id=$(echo "$body" | json_str client_id)
  secret=$(echo "$body" | json_str client_secret)
  if [ -z "$client_id" ] || [ -z "$secret" ]; then
    log "FATAL: client credentials not available"
    return 1
  fi
  kubectl -n "$K8S_NAMESPACE" create secret generic "$OIDC_CLIENT_SECRET_K8S" \
    --from-literal=client_id="$client_id" \
    --from-literal=client_secret="$secret" \
    --dry-run=client -o yaml | kubectl apply -f -
}

seed_admin_user() {
  if [ -z "${ADMIN_SEED_USER:-}" ] || [ -z "${ADMIN_SEED_PASSWORD:-}" ]; then
    log "ADMIN_SEED_USER/ADMIN_SEED_PASSWORD not set — skipping admin seed"
    return 0
  fi
  code=$(api_code GET "/v1/auth/userpass/users/$ADMIN_SEED_USER")
  if [ "$code" = "200" ]; then
    log "admin user $ADMIN_SEED_USER already exists — not resetting password"
  else
    log "creating userpass user $ADMIN_SEED_USER"
    printf '{"password":"%s"}' "$ADMIN_SEED_PASSWORD" > /tmp/user.json
    api PUT "/v1/auth/userpass/users/$ADMIN_SEED_USER" /tmp/user.json >/dev/null
  fi
  entity_id=$(api GET "/v1/identity/entity/name/$ADMIN_SEED_USER" | data_id)
  if [ -z "$entity_id" ]; then
    printf '{"name":"%s"}' "$ADMIN_SEED_USER" > /tmp/e.json
    entity_id=$(api POST "/v1/identity/entity" /tmp/e.json | data_id)
  fi
  if [ -z "$entity_id" ]; then
    log "FATAL: could not resolve entity_id for $ADMIN_SEED_USER"
    return 1
  fi
  userpass_acc=$(api GET "/v1/sys/auth" | grep -o '"userpass/":{[^}]*' | grep -o '"accessor":"[^"]*"' | head -1 | cut -d'"' -f4)
  printf '{"name":"%s","canonical_id":"%s","mount_accessor":"%s"}' \
    "$ADMIN_SEED_USER" "$entity_id" "$userpass_acc" > /tmp/ea.json
  api POST "/v1/identity/entity-alias" /tmp/ea.json >/dev/null
  admin_gid=$(api GET "/v1/identity/group/name/admin" | data_id)
  if [ -z "$admin_gid" ]; then
    log "FATAL: could not resolve admin group id"
    return 1
  fi
  # OpenBao has been observed to accept the PUT but leave
  # member_entity_ids null on retry; verify the read-back before
  # declaring success so a broken assignment surfaces here.
  printf '{"member_entity_ids":["%s"]}' "$entity_id" > /tmp/mg.json
  i=0
  while true; do
    api PUT "/v1/identity/group/id/$admin_gid" /tmp/mg.json >/dev/null
    if api GET "/v1/identity/group/id/$admin_gid" | grep -q "\"$entity_id\""; then
      log "admin entity $entity_id in admin group $admin_gid (verified)"
      return 0
    fi
    i=$((i + 1))
    if [ "$i" -ge 5 ]; then
      log "FATAL: admin group membership did not stick after $i attempts"
      api GET "/v1/identity/group/id/$admin_gid" >&2
      return 1
    fi
    log "admin group membership not yet reflected (attempt $i) — retrying"
    sleep 1
  done
}

verify_discovery() {
  log "verifying discovery document"
  doc=$(curl -sS "$BAO_ADDR/v1/identity/oidc/provider/nightshift/.well-known/openid-configuration")
  if echo "$doc" | grep -q '"authorization_endpoint"'; then
    log "discovery ok"
  else
    log "FATAL: discovery document missing authorization_endpoint"
    echo "$doc" >&2
    return 1
  fi
}

main() {
  : "${BAO_ADDR:?}"
  : "${BAO_TOKEN:?}"
  : "${OIDC_ISSUER:?}"
  : "${OIDC_CLIENT_NAME:?}"
  : "${CLIENT_REDIRECT_URI:?}"
  : "${K8S_NAMESPACE:?}"
  : "${API_SA_NAME:?}"
  : "${WORKER_SA_NAME:?}"
  OIDC_CLIENT_SECRET_K8S="${OIDC_CLIENT_SECRET_K8S:-openbao-oidc-client}"

  wait_for_unseal
  enable_userpass
  enable_kv_v2
  enable_kubernetes_auth
  ensure_policy nightshift-api    /scripts/nightshift-api-policy.hcl
  ensure_policy nightshift-worker /scripts/nightshift-worker-policy.hcl
  ensure_policy nightshift-admin  /scripts/nightshift-admin-policy.hcl
  ensure_k8s_role nightshift-api    "$API_SA_NAME"    nightshift-api
  ensure_k8s_role nightshift-worker "$WORKER_SA_NAME" nightshift-worker
  # admin group carries the nightshift-admin policy so members can
  # manage userpass + identity (create new users, add them to groups,
  # rotate passwords) without needing the OpenBao root token.
  ensure_group admin "nightshift-admin"
  ensure_group user  ""
  ensure_key
  ensure_scope
  ensure_assignment
  ensure_client
  if [ -n "${UI_CLIENT_REDIRECT_URI:-}" ]; then
    ensure_ui_client
  fi
  ensure_provider
  publish_client_secret_to_k8s
  if [ -n "${UI_CLIENT_REDIRECT_URI:-}" ]; then
    publish_ui_client_secret_to_k8s
  fi
  seed_admin_user
  verify_discovery
  log "bootstrap complete"
}

main "$@"
