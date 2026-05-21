{{/*
Fullname for a component. Scoped per release:
  {{ include "nightshift.fullname" (dict "Release" .Release "component" "nightshift-api") }}
*/}}
{{- define "nightshift.fullname" -}}
{{- printf "%s-%s" .Release.Name .component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every object. Component-specific labels are
added at the call site.
*/}}
{{- define "nightshift.labels" -}}
app.kubernetes.io/name: nightshift
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: nightshift
{{- if .Values.tenant.name }}
tenant: {{ .Values.tenant.name }}
{{- end }}
{{- end }}

{{/*
Selector labels for a component. Call as:
  {{ include "nightshift.selectorLabels" (dict "Release" .Release "component" "nightshift-api") }}
*/}}
{{- define "nightshift.selectorLabels" -}}
app.kubernetes.io/name: nightshift
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Image reference for the nightshift-api container.
*/}}
{{- define "nightshift.apiImage" -}}
{{- $defaultTag := default .Chart.AppVersion .Values.image.tag -}}
{{- $tag := default $defaultTag .Values.nightshift_api.image.tag -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.nightshift_api.image.repository $tag }}
{{- end }}

{{/*
Image reference for the worker the API launches per run.
*/}}
{{- define "nightshift.workerImage" -}}
{{- $defaultTag := default .Chart.AppVersion .Values.image.tag -}}
{{- $tag := default $defaultTag .Values.nightshift_api.worker.tag -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.nightshift_api.worker.repository $tag }}
{{- end }}

{{/*
Worker-launch namespace. Falls back to the release namespace.
*/}}
{{- define "nightshift.workerNamespace" -}}
{{- default .Release.Namespace .Values.nightshift_api.kube.namespace }}
{{- end }}

{{/*
Bundled MinIO resource name. Released as <release>-minio.
*/}}
{{- define "nightshift.minioFullname" -}}
{{- printf "%s-minio" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Cluster-internal endpoint URL for the bundled MinIO. Used both as
NS_S3_ENDPOINT and (by default) NS_S3_PRESIGN_ENDPOINT.
*/}}
{{- define "nightshift.minioEndpoint" -}}
{{- printf "http://%s.%s.svc:9000" (include "nightshift.minioFullname" .) .Release.Namespace }}
{{- end }}

{{/*
Bundled Postgres resource name. Released as <release>-postgres.
*/}}
{{- define "nightshift.postgresFullname" -}}
{{- printf "%s-postgres" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Cluster-internal Postgres DSN for the bundled Postgres deployment.
Password is mounted from the chart's postgres Secret at runtime via
secretKeyRef, so this template does NOT include the password — it
emits a DSN with a placeholder that env-var substitution fills in.
The API container runs with the secret-key as a separate env var
named POSTGRES_PASSWORD; NS_POSTGRES_DSN is built from these via
shell expansion in the Deployment's env.
*/}}
{{- define "nightshift.postgresHost" -}}
{{- printf "%s.%s.svc" (include "nightshift.postgresFullname" .) .Release.Namespace }}
{{- end }}

{{/*
Postgres connection env entries. Pods compose the DSN at start-time
from the live nightshift-postgres Secret using K8s $(VAR)
substitution, so a placeholder password in values.yaml never reaches
the rendered DSN. Caller passes a dict { "dsnVar": "<env-name>",
"context": $ } to choose the DSN env name (NS_POSTGRES_DSN for the
API, DATABASE_URL for the UI).
*/}}
{{- define "nightshift.postgresEnv" -}}
- name: POSTGRES_USER
  valueFrom:
    secretKeyRef:
      name: {{ include "nightshift.postgresFullname" .context }}
      key: POSTGRES_USER
- name: POSTGRES_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "nightshift.postgresFullname" .context }}
      key: POSTGRES_PASSWORD
- name: POSTGRES_DB
  valueFrom:
    secretKeyRef:
      name: {{ include "nightshift.postgresFullname" .context }}
      key: POSTGRES_DB
- name: {{ .dsnVar }}
  value: "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@{{ include "nightshift.postgresHost" .context }}:5432/$(POSTGRES_DB)?sslmode=disable"
{{- end }}

{{/*
Bundled OpenBao Service name. Stable at "openbao" so it matches the
bootstrap script's references and any existing operator runbooks.
*/}}
{{- define "nightshift.openbaoFullname" -}}
openbao
{{- end }}

{{/*
Cluster-internal API endpoint for OpenBao. Used by the bootstrap Job
and the API for OIDC discovery / token verification.
*/}}
{{- define "nightshift.openbaoEndpoint" -}}
{{- printf "http://%s.%s.svc:8200" (include "nightshift.openbaoFullname" .) .Release.Namespace }}
{{- end }}

{{/*
Worker environment passthrough — JSON-encoded map[string]string the
API forwards onto every worker pod via NS_WORKER_ENV. The API/worker
binaries are intentionally blind to which secrets backend is in play;
this helper is where chart-level knowledge of backend wiring lives.

Layering: chart-derived defaults are computed first, then user-supplied
`nightshift_api.worker.env` is merged ON TOP (user wins on collision).

Defaults today: when OpenBao is bundled AND the Python claude worker
is enabled, inject the OpenBao address + auth role the worker needs
for its Kubernetes-auth login. With either flag off, the helper emits
only the user's overrides (or `{}`).
*/}}
{{- define "nightshift.workerEnv" -}}
{{- $env := dict -}}
{{- if and .Values.openbao.enabled .Values.nightshift_api.workerClaude.enabled -}}
  {{- $_ := set $env "NS_OPENBAO_ADDR" (include "nightshift.openbaoEndpoint" .) -}}
  {{- $_ := set $env "NS_OPENBAO_AUTH_ROLE" "nightshift-worker" -}}
{{- end -}}
{{- range $k, $v := .Values.nightshift_api.worker.env -}}
  {{- $_ := set $env $k $v -}}
{{- end -}}
{{- $env | toJson -}}
{{- end }}

{{/*
OIDC issuer base URL — scheme + host + optional port, no path. This
is what's PUT into the OpenBao provider config; OpenBao rejects any
URL with a path component (it appends the canonical
/v1/identity/oidc/provider/<name> path itself). Operators override
with the externally-resolvable hostname for production deployments;
in-cluster default is the OpenBao Service.
*/}}
{{- define "nightshift.openbaoIssuerBase" -}}
{{- if .Values.openbao.oidc.issuerURL -}}
{{- .Values.openbao.oidc.issuerURL -}}
{{- else -}}
{{- include "nightshift.openbaoEndpoint" . -}}
{{- end -}}
{{- end }}

{{/*
Full OIDC issuer URL — base + /v1/identity/oidc/provider/nightshift.
This is the value that appears in JWT `iss` claims and at which
discovery is served, so it's what the API consumes as
NS_OIDC_ISSUER_URL.
*/}}
{{- define "nightshift.openbaoIssuerURL" -}}
{{- printf "%s/v1/identity/oidc/provider/nightshift" (include "nightshift.openbaoIssuerBase" .) -}}
{{- end }}

{{/*
Name of the K8s Secret the bootstrap Job writes with client_id +
client_secret for the OIDC client.
*/}}
{{- define "nightshift.openbaoOidcClientSecret" -}}
{{- default "openbao-oidc-client" .Values.openbao.oidc.clientSecretRef }}
{{- end }}

{{/*
Name of the K8s Secret holding OpenBao's unseal key + root token.
Pre-created as a placeholder, populated by openbao-init.
*/}}
{{- define "nightshift.openbaoSeedSecret" -}}
openbao-seed
{{- end }}

{{/*
Name of the K8s Secret optionally seeding an admin userpass user.
*/}}
{{- define "nightshift.openbaoAdminSeedSecret" -}}
openbao-admin-seed
{{- end }}

{{/*
Image reference for the UI container (chunk 19).
*/}}
{{- define "nightshift.uiImage" -}}
{{- $defaultTag := default .Chart.AppVersion .Values.image.tag -}}
{{- $tag := default $defaultTag .Values.ui.image.tag -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.ui.image.repository $tag }}
{{- end }}

{{/*
Name of the K8s Secret the bootstrap Job writes with client_id +
client_secret for the chunk-19 UI's OIDC client. Distinct from the
nightshift-api client so credentials don't leak across components.
*/}}
{{- define "nightshift.openbaoOidcUiClientSecret" -}}
{{- default "openbao-oidc-ui-client" .Values.openbao.oidc.uiClientSecretRef }}
{{- end }}

{{/*
The UI's better-auth-facing public hostname. Used both as
BETTER_AUTH_URL + as the OIDC redirect URI suffix. Operator-supplied;
falls back to the Service DNS for in-cluster-only smoke.
*/}}
{{- define "nightshift.uiHost" -}}
{{- if .Values.ui.ingress.host -}}
{{- .Values.ui.ingress.host -}}
{{- else -}}
{{- printf "%s-ui.%s.svc" .Release.Name .Release.Namespace -}}
{{- end -}}
{{- end }}

{{/*
Browser-facing OpenBao base URL. The UI's OIDC_AUTHORIZATION_URL
(the redirect target the browser follows) needs to point here, while
token + userinfo URLs (server-side calls) stay on the in-cluster
service. Resolution order:
  1. openbao.oidc.publicEndpoint (operator override; required for dev
     port-forward where the browser can't resolve in-cluster DNS)
  2. openbao.oidc.issuerURL (the canonical issuer; same hostname is
     used for browser and in-cluster when an ingress exposes openbao)
  3. nightshift.openbaoEndpoint (in-cluster Service URL — works only
     when the browser is also in the cluster, e.g. another pod)
*/}}
{{- define "nightshift.openbaoPublicEndpoint" -}}
{{- if .Values.openbao.oidc.publicEndpoint -}}
{{- .Values.openbao.oidc.publicEndpoint -}}
{{- else if .Values.openbao.oidc.issuerURL -}}
{{- .Values.openbao.oidc.issuerURL -}}
{{- else -}}
{{- include "nightshift.openbaoEndpoint" . -}}
{{- end -}}
{{- end }}

{{/*
Browser-facing UI URL — full scheme + host + optional port. Used as
BETTER_AUTH_URL on the UI pod + as the base of the OIDC redirect URI
registered with OpenBao. Resolution order:
  1. ui.publicURL (operator override; required for dev port-forward)
  2. https://<ui.ingress.host> when ingress is configured
  3. https://<service-DNS> as a final fallback
*/}}
{{- define "nightshift.uiPublicURL" -}}
{{- if .Values.ui.publicURL -}}
{{- .Values.ui.publicURL -}}
{{- else -}}
{{- printf "https://%s" (include "nightshift.uiHost" .) -}}
{{- end -}}
{{- end }}

{{/*
Better-auth's OIDC callback path is `/api/auth/oauth2/callback/<provider>`
where the provider name is "openbao" in our config. The OIDC client's
redirect_uri registered in OpenBao must match this exactly, otherwise
the OAuth code-exchange step rejects with `invalid_redirect_uri`.
*/}}
{{- define "nightshift.uiOidcCallbackURL" -}}
{{- printf "%s/api/auth/oauth2/callback/openbao" (include "nightshift.uiPublicURL" .) -}}
{{- end }}
