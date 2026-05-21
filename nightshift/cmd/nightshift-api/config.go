package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nightshiftco/nightshift/internal/objects"
)

// Config holds the runtime configuration for nightshift-api.
// Populated from NS_* environment variables.
type Config struct {
	GRPCAddr    string // NS_GRPC_ADDR (default :50051)
	HTTPAddr    string // NS_HTTP_ADDR (default :8080)
	MetricsAddr string // NS_METRICS_ADDR (default :9090) — Prometheus scrape

	// RecordsBackend selects the records.RecordStore implementation:
	// "sqlite" (default) or "postgres".
	RecordsBackend string // NS_RECORDS_BACKEND

	// DBPath is the SQLite file path (RecordsBackend=sqlite). Empty
	// or "memory" means in-memory SQLite (dev only).
	DBPath string // NS_DB_PATH

	// PostgresDSN is the Postgres connection string
	// (RecordsBackend=postgres). Standard libpq URL form, e.g.
	// postgres://user:pw@host:5432/nightshift?sslmode=require.
	PostgresDSN string // NS_POSTGRES_DSN

	// MigrateOnly, when true, exits the process after the records
	// backend's Open* applies its schema. Used by the helm chart's
	// pre-install Job hook to apply Postgres schema once before N
	// replicas of the API Deployment start. No-op for sqlite (the
	// chart doesn't render the Job in that mode).
	MigrateOnly bool // NS_MIGRATE_ONLY

	// Object storage.
	ObjectsBackend string // NS_OBJECTS_BACKEND: filesystem (default) | s3
	ObjectsDir     string // NS_OBJECTS_DIR (filesystem)
	ObjectsBaseURL string // NS_OBJECTS_BASE_URL (filesystem; defaults to <HTTP>/_objects)

	// S3 backend (when ObjectsBackend=="s3").
	S3Endpoint        string // NS_S3_ENDPOINT (e.g. http://minio.ns.svc:9000)
	S3PresignEndpoint string // NS_S3_PRESIGN_ENDPOINT (defaults to S3Endpoint)
	S3Region          string // NS_S3_REGION (default us-east-1)
	S3Bucket          string // NS_S3_BUCKET
	S3AccessKeyID     string // NS_S3_ACCESS_KEY_ID
	S3SecretAccessKey string // NS_S3_SECRET_ACCESS_KEY
	S3UsePathStyle    bool   // NS_S3_USE_PATH_STYLE (default true; required for MinIO)

	// Secrets backend.
	SecretsFile string // NS_SECRETS_FILE (YAML)

	// HMAC secret used to sign filesystem presigned URLs.
	ObjectsHMACSecret string // NS_OBJECTS_HMAC

	// Workers runtime.
	Runtime        string // NS_RUNTIME: stub | kubernetes (default stub)
	WorkerImage    string // NS_WORKER_IMAGE (required for kubernetes runtime)
	WorkerBinary   string // NS_WORKER_BINARY (required for stub runtime)
	WorkerCallback string // NS_WORKER_CALLBACK (default: grpc://<NS_GRPC_ADDR>)
	KubeconfigPath string // NS_KUBECONFIG ("" in-cluster, "auto" default-loading, or a path)
	KubeNamespace  string // NS_KUBE_NAMESPACE (default: nightshift)
	KubeWorkerSA   string // NS_KUBE_WORKER_SA (default: nightshift-worker)

	// Auth.
	OIDCIssuerURL    string   // NS_OIDC_ISSUER_URL (empty disables OIDC)
	OIDCAudience     string   // NS_OIDC_AUDIENCE (required when OIDC is enabled)
	StaticTokensPath string   // NS_STATIC_TOKENS_PATH (default secret/nightshift/static-tokens)
	AuthAdminGroup   string   // NS_AUTH_ADMIN_GROUP (OIDC group claim that grants admin; default "admin")
	AuthAdminTokens  []string // NS_AUTH_ADMIN_TOKENS (comma-separated static-token names that grant admin)

	// Secrets backend selection.
	SecretsBackend  string // NS_SECRETS_BACKEND: file (default) | openbao
	OpenBaoAddr     string // NS_OPENBAO_ADDR (default http://openbao.<release-namespace>.svc:8200)
	OpenBaoAuthRole string // NS_OPENBAO_AUTH_ROLE (default nightshift-api)
	OpenBaoKVMount  string // NS_OPENBAO_KV_MOUNT (default secret)

	// OpenBaoUsersGroup is the OpenBao identity group whose
	// member entities back the chunk-19 share-dialog user picker.
	// Empty disables /v1/users (UI proxy returns []). Default "user".
	OpenBaoUsersGroup string // NS_OPENBAO_USERS_GROUP (default user)

	// Connector catalog.
	ConnectorCatalogFile string // NS_CONNECTOR_CATALOG_FILE (empty uses embedded default)

	// Worker pod hardening (chunks 9 + 14).
	WorkerMountSAToken bool // NS_WORKER_MOUNT_SA_TOKEN — flip true for nightshift-worker-claude (OpenBao K8s-auth)

	// WorkerEnv is a chart-driven, opaque map of env vars the API
	// forwards onto every worker pod. Set via NS_WORKER_ENV (a JSON
	// object: {"NS_FOO":"bar","NS_BAZ":"qux"}). The API does not
	// interpret keys or values — this is the passthrough seam that
	// keeps backend-specific worker config (e.g. the secrets-store
	// address for the Python claude worker's OpenBao login) out of
	// the API binary. Empty / unset is fine.
	WorkerEnv map[string]string // NS_WORKER_ENV (JSON object)

	// Artifacts (chunk 15). Bucket name to use for artifact blobs +
	// companion previews. Defaults to "artifacts" — separate from the
	// chunk-13 session-state bucket so cleanup boundaries are clear.
	ArtifactsBucket string // NS_ARTIFACTS_BUCKET (default artifacts)

	// App artifacts (chunk 16).
	AppDeployNamespace string        // NS_APP_DEPLOY_NAMESPACE (defaults to NS_KUBE_NAMESPACE)
	AppNginxImage      string        // NS_APP_NGINX_IMAGE (default nginxinc/nginx-unprivileged:1-alpine)
	AppInitImage       string        // NS_APP_INIT_IMAGE (default curlimages/curl:latest)
	AppDownloadTTL     time.Duration // NS_APP_DOWNLOAD_TTL (default 30m)

	// Scheduling (chunk 17). The CronJob fired by each schedule POSTs
	// CreateRun to APIInternalURL with the bearer in TokenSecret.
	APIInternalURL           string // NS_API_INTERNAL_URL (auto-derived from in-cluster service DNS if empty)
	SchedulerFireImage       string // NS_SCHEDULER_FIRE_IMAGE (default curlimages/curl:latest)
	SchedulerTokenSecretName string // NS_SCHEDULER_TOKEN_SECRET_NAME (default nightshift-scheduler-token)

	// Per-session state volume (chunk 13).
	SessionStateBackend      string // NS_SESSION_STATE_BACKEND: none (default) | pvc | host | object
	SessionStateMountPath    string // NS_SESSION_STATE_MOUNT_PATH (default /var/lib/nightshift/session-state)
	SessionStatePVCName      string // NS_SESSION_STATE_PVC_NAME (pvc backend)
	SessionStateHostRoot     string // NS_SESSION_STATE_HOST_ROOT (host backend)
	SessionStateObjectBucket string // NS_SESSION_STATE_OBJECT_BUCKET (object backend; defaults to NS_S3_BUCKET)

	// Drain / timeouts.
	DrainTimeout time.Duration // NS_DRAIN_TIMEOUT (default 15s)
}

// LoadConfig reads env vars and applies defaults. Returns an error if
// required fields are missing or malformed.
func LoadConfig() (*Config, error) {
	c := &Config{
		GRPCAddr:                 env("NS_GRPC_ADDR", ":50051"),
		HTTPAddr:                 env("NS_HTTP_ADDR", ":8080"),
		MetricsAddr:              env("NS_METRICS_ADDR", ":9090"),
		RecordsBackend:           env("NS_RECORDS_BACKEND", "sqlite"),
		DBPath:                   env("NS_DB_PATH", ""),
		PostgresDSN:              env("NS_POSTGRES_DSN", ""),
		MigrateOnly:              envBool("NS_MIGRATE_ONLY", false),
		ObjectsBackend:           env("NS_OBJECTS_BACKEND", "filesystem"),
		ObjectsDir:               env("NS_OBJECTS_DIR", ""),
		ObjectsBaseURL:           env("NS_OBJECTS_BASE_URL", ""),
		S3Endpoint:               env("NS_S3_ENDPOINT", ""),
		S3PresignEndpoint:        env("NS_S3_PRESIGN_ENDPOINT", ""),
		S3Region:                 env("NS_S3_REGION", "us-east-1"),
		S3Bucket:                 env("NS_S3_BUCKET", ""),
		S3AccessKeyID:            env("NS_S3_ACCESS_KEY_ID", ""),
		S3SecretAccessKey:        env("NS_S3_SECRET_ACCESS_KEY", ""),
		S3UsePathStyle:           envBool("NS_S3_USE_PATH_STYLE", true),
		SecretsFile:              env("NS_SECRETS_FILE", ""),
		ObjectsHMACSecret:        env("NS_OBJECTS_HMAC", ""),
		Runtime:                  env("NS_RUNTIME", "stub"),
		WorkerImage:              env("NS_WORKER_IMAGE", ""),
		WorkerBinary:             env("NS_WORKER_BINARY", ""),
		WorkerCallback:           env("NS_WORKER_CALLBACK", ""),
		KubeconfigPath:           env("NS_KUBECONFIG", ""),
		KubeNamespace:            env("NS_KUBE_NAMESPACE", "nightshift"),
		KubeWorkerSA:             env("NS_KUBE_WORKER_SA", "nightshift-worker"),
		OIDCIssuerURL:            env("NS_OIDC_ISSUER_URL", ""),
		OIDCAudience:             env("NS_OIDC_AUDIENCE", ""),
		StaticTokensPath:         env("NS_STATIC_TOKENS_PATH", "secret/nightshift/static-tokens"),
		AuthAdminGroup:           env("NS_AUTH_ADMIN_GROUP", "admin"),
		AuthAdminTokens:          parseCSV(env("NS_AUTH_ADMIN_TOKENS", "")),
		SecretsBackend:           env("NS_SECRETS_BACKEND", "file"),
		OpenBaoAddr:              env("NS_OPENBAO_ADDR", ""),
		OpenBaoAuthRole:          env("NS_OPENBAO_AUTH_ROLE", "nightshift-api"),
		OpenBaoKVMount:           env("NS_OPENBAO_KV_MOUNT", "secret"),
		OpenBaoUsersGroup:        env("NS_OPENBAO_USERS_GROUP", "user"),
		ConnectorCatalogFile:     env("NS_CONNECTOR_CATALOG_FILE", ""),
		WorkerMountSAToken:       envBool("NS_WORKER_MOUNT_SA_TOKEN", false),
		ArtifactsBucket:          env("NS_ARTIFACTS_BUCKET", "artifacts"),
		AppDeployNamespace:       env("NS_APP_DEPLOY_NAMESPACE", ""),
		AppNginxImage:            env("NS_APP_NGINX_IMAGE", "nginxinc/nginx-unprivileged:1-alpine"),
		AppInitImage:             env("NS_APP_INIT_IMAGE", "curlimages/curl:latest"),
		APIInternalURL:           env("NS_API_INTERNAL_URL", ""),
		SchedulerFireImage:       env("NS_SCHEDULER_FIRE_IMAGE", "curlimages/curl:latest"),
		SchedulerTokenSecretName: env("NS_SCHEDULER_TOKEN_SECRET_NAME", "nightshift-scheduler-token"),
		SessionStateBackend:      env("NS_SESSION_STATE_BACKEND", "none"),
		SessionStateMountPath:    env("NS_SESSION_STATE_MOUNT_PATH", "/var/lib/nightshift/session-state"),
		SessionStatePVCName:      env("NS_SESSION_STATE_PVC_NAME", ""),
		SessionStateHostRoot:     env("NS_SESSION_STATE_HOST_ROOT", "/var/lib/nightshift/session-state"),
		SessionStateObjectBucket: env("NS_SESSION_STATE_OBJECT_BUCKET", ""),
	}

	if raw := os.Getenv("NS_WORKER_ENV"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &c.WorkerEnv); err != nil {
			return nil, fmt.Errorf("invalid NS_WORKER_ENV (expected JSON object): %w", err)
		}
	}

	if d := os.Getenv("NS_DRAIN_TIMEOUT"); d != "" {
		dur, err := time.ParseDuration(d)
		if err != nil {
			return nil, errors.New("invalid NS_DRAIN_TIMEOUT: " + err.Error())
		}
		c.DrainTimeout = dur
	} else {
		c.DrainTimeout = 15 * time.Second
	}

	if d := os.Getenv("NS_APP_DOWNLOAD_TTL"); d != "" {
		dur, err := time.ParseDuration(d)
		if err != nil {
			return nil, errors.New("invalid NS_APP_DOWNLOAD_TTL: " + err.Error())
		}
		c.AppDownloadTTL = dur
	} else {
		c.AppDownloadTTL = 30 * time.Minute
	}

	switch c.RecordsBackend {
	case "", "sqlite":
		c.RecordsBackend = "sqlite"
	case "postgres":
		if c.PostgresDSN == "" {
			return nil, errors.New("NS_POSTGRES_DSN required when NS_RECORDS_BACKEND=postgres")
		}
	default:
		return nil, fmt.Errorf("unknown NS_RECORDS_BACKEND=%q (expected sqlite|postgres)", c.RecordsBackend)
	}

	// MigrateOnly short-circuits the rest of validation: the binary
	// will only Open the records backend (which applies schema) and
	// exit. It doesn't need objects, workers, runtime, sessionState,
	// scheduling, etc. Pre-install hook Jobs run with a stripped-down
	// env (records-only); validating the full surface would force
	// them to mount creds they don't use.
	if c.MigrateOnly {
		return c, nil
	}

	switch c.ObjectsBackend {
	case "filesystem":
		if c.ObjectsDir == "" {
			c.ObjectsDir = "/var/lib/nightshift/objects"
		}
		if c.ObjectsHMACSecret == "" {
			return nil, errors.New("NS_OBJECTS_HMAC is required for filesystem backend (>= 16 bytes)")
		}
		if c.ObjectsBaseURL == "" {
			c.ObjectsBaseURL = inferObjectsBaseURL(c.HTTPAddr)
		}
	case "s3":
		if c.S3Endpoint == "" {
			return nil, errors.New("NS_S3_ENDPOINT is required for s3 backend")
		}
		if c.S3Bucket == "" {
			return nil, errors.New("NS_S3_BUCKET is required for s3 backend")
		}
		if c.S3AccessKeyID == "" || c.S3SecretAccessKey == "" {
			return nil, errors.New("NS_S3_ACCESS_KEY_ID and NS_S3_SECRET_ACCESS_KEY are required for s3 backend")
		}
	default:
		return nil, errors.New("NS_OBJECTS_BACKEND must be 'filesystem' or 's3'")
	}

	if c.WorkerCallback == "" {
		c.WorkerCallback = inferGRPCCallback(c.GRPCAddr)
	}

	switch c.Runtime {
	case "stub":
		if c.WorkerBinary == "" {
			return nil, errors.New("NS_WORKER_BINARY is required when NS_RUNTIME=stub")
		}
	case "kubernetes":
		if c.WorkerImage == "" {
			return nil, errors.New("NS_WORKER_IMAGE is required when NS_RUNTIME=kubernetes")
		}
	default:
		return nil, errors.New("NS_RUNTIME must be 'stub' or 'kubernetes'")
	}

	switch c.SessionStateBackend {
	case "", "none":
		c.SessionStateBackend = "none"
	case "pvc":
		if c.SessionStatePVCName == "" {
			return nil, errors.New("NS_SESSION_STATE_PVC_NAME is required when NS_SESSION_STATE_BACKEND=pvc")
		}
	case "host":
		if c.SessionStateHostRoot == "" {
			return nil, errors.New("NS_SESSION_STATE_HOST_ROOT is required when NS_SESSION_STATE_BACKEND=host")
		}
	case "object":
		if c.SessionStateObjectBucket == "" {
			c.SessionStateObjectBucket = c.S3Bucket
		}
		if c.SessionStateObjectBucket == "" {
			return nil, errors.New("NS_SESSION_STATE_OBJECT_BUCKET (or NS_S3_BUCKET) is required when NS_SESSION_STATE_BACKEND=object")
		}
	default:
		return nil, errors.New("NS_SESSION_STATE_BACKEND must be 'none', 'pvc', 'host', or 'object'")
	}

	return c, nil
}

// inferGRPCCallback converts a listen addr like ":50051" or
// "0.0.0.0:50051" into a callback URL workers can dial inside the
// cluster. Operators SHOULD override in production with the Service
// DNS name (e.g. "nightshift-api.nightshift.svc:50051").
func inferGRPCCallback(grpcAddr string) string {
	host := "127.0.0.1"
	port := "50051"
	for i := len(grpcAddr) - 1; i >= 0; i-- {
		if grpcAddr[i] == ':' {
			if grpcAddr[:i] != "" && grpcAddr[:i] != "0.0.0.0" {
				host = grpcAddr[:i]
			}
			port = grpcAddr[i+1:]
			break
		}
	}
	return host + ":" + port
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// parseCSV splits a comma-separated env-var value into trimmed, non-empty
// names. Used for NS_AUTH_ADMIN_TOKENS.
func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// toS3Config builds an objects.S3Config from the parsed env config.
// Caller must ensure ObjectsBackend == "s3".
func (c *Config) toS3Config() objects.S3Config {
	return objects.S3Config{
		Endpoint:        c.S3Endpoint,
		PresignEndpoint: c.S3PresignEndpoint,
		Region:          c.S3Region,
		Bucket:          c.S3Bucket,
		AccessKeyID:     c.S3AccessKeyID,
		SecretAccessKey: c.S3SecretAccessKey,
		UsePathStyle:    c.S3UsePathStyle,
	}
}

// inferObjectsBaseURL returns a sensible default presign URL base
// from the HTTP listen addr. Operators MUST override in production.
func inferObjectsBaseURL(httpAddr string) string {
	host := "127.0.0.1"
	port := "8080"
	// parse "host:port" roughly
	for i := len(httpAddr) - 1; i >= 0; i-- {
		if httpAddr[i] == ':' {
			if httpAddr[:i] != "" {
				host = httpAddr[:i]
			}
			port = httpAddr[i+1:]
			break
		}
	}
	return "http://" + host + ":" + port + "/_objects"
}
