// Command nightshift-api is the Go reference implementation of the
// Nightshift specification. This binary starts the gRPC server, the
// grpc-gateway HTTP server, and the embedded presign handler.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	artifactssvc "github.com/nightshiftco/nightshift/internal/api/artifacts"
	authapisvc "github.com/nightshiftco/nightshift/internal/api/authapi"
	configsvc "github.com/nightshiftco/nightshift/internal/api/config"
	schedulingsvc "github.com/nightshiftco/nightshift/internal/api/scheduling"
	secretsapisvc "github.com/nightshiftco/nightshift/internal/api/secretsapi"
	sessionstatesvc "github.com/nightshiftco/nightshift/internal/api/sessionstate"
	storagesvc "github.com/nightshiftco/nightshift/internal/api/storage"
	"github.com/nightshiftco/nightshift/internal/api/users"
	workerssvc "github.com/nightshiftco/nightshift/internal/api/workers"
	"github.com/nightshiftco/nightshift/internal/broadcaster"
	"github.com/nightshiftco/nightshift/internal/identity"
	"github.com/nightshiftco/nightshift/internal/metrics"
	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/objects"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
	"github.com/nightshiftco/nightshift/internal/secrets"
	"github.com/nightshiftco/nightshift/internal/server"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// workerHMACPath is the convention path under which the Secrets
// backend supplies the HMAC key used to sign worker credentials.
// The YAML/env secrets backend accepts either an entry at this key
// or the env-var fallback NS_SECRET_SECRET_NIGHTSHIFT_WORKER_HMAC.
const workerHMACPath = "secret/nightshift/worker-hmac"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	var recStore records.RecordStore
	switch cfg.RecordsBackend {
	case "postgres":
		logger.Info("records backend=postgres")
		pg, err := records.OpenPostgres(cfg.PostgresDSN)
		if err != nil {
			return fmt.Errorf("records postgres: %w", err)
		}
		recStore = pg
	default: // "" or "sqlite" — config validation already normalizes
		dsn := cfg.DBPath
		if dsn == "" || dsn == "memory" {
			dsn = "file::memory:?cache=shared"
		} else {
			dsn = "file:" + dsn
		}
		logger.Info("records backend=sqlite", "dsn", dsn)
		sq, err := records.OpenSQLite(dsn)
		if err != nil {
			return fmt.Errorf("records sqlite: %w", err)
		}
		recStore = sq
	}
	defer func() { _ = recStore.Close() }()

	// NS_MIGRATE_ONLY: schema is applied as a side effect of the
	// records-backend Open* call above. Exit before any other
	// initialization (no listeners, no stale-run recovery, no
	// session-state setup). Used by the chart's pre-install Job hook
	// so a single pod applies schema once before N API replicas start.
	if cfg.MigrateOnly {
		logger.Info("migrate-only: schema applied, exiting", "backend", cfg.RecordsBackend)
		return nil
	}

	if n, err := recStore.RecoverStaleRuns(context.Background()); err != nil {
		logger.Warn("stale-run recovery failed", "err", err)
	} else if n > 0 {
		logger.Info("stale-run recovery", "transitioned", n)
	}

	var objStore objects.ObjectStore
	switch cfg.ObjectsBackend {
	case "s3":
		s3, err := objects.NewS3(context.Background(), cfg.toS3Config())
		if err != nil {
			return fmt.Errorf("objects s3: %w", err)
		}
		objStore = s3
	default:
		return fmt.Errorf("unknown NS_OBJECTS_BACKEND=%q", cfg.ObjectsBackend)
	}

	// File-backed secrets always loaded — bootstrap secrets (worker
	// HMAC, static tokens) live there in chunk 10/11. Per-user
	// connector tokens (chunk 11 SetConnectorStaticToken) require a
	// writable backend; OpenBao is the only one today.
	fileSecrets, err := secrets.NewFile(cfg.SecretsFile)
	if err != nil {
		return fmt.Errorf("secrets file: %w", err)
	}
	workerHMAC, err := loadWorkerHMAC(fileSecrets)
	if err != nil {
		return fmt.Errorf("worker hmac: %w", err)
	}

	// Secrets backend used by the Config service for per-user
	// connector credentials. file backend's Put returns
	// ErrNotImplemented — operators wanting writable per-user creds
	// (and OAuth) must enable openbao. The OAuthDispenser is always
	// Native (in-process PKCE/S256), parameterized by whichever
	// secrets backend is selected.
	configSecrets, oauthDispenser, identityDir, err := buildConfigSecrets(cfg, fileSecrets, logger)
	if err != nil {
		return fmt.Errorf("config secrets backend: %w", err)
	}

	bootCtx := context.Background()
	staticVer, err := verifiers.LoadStaticVerifier(bootCtx, fileSecrets, cfg.StaticTokensPath)
	if err != nil {
		return fmt.Errorf("static tokens: %w", err)
	}
	oidcVer, err := verifiers.NewOIDCVerifier(bootCtx, cfg.OIDCIssuerURL, cfg.OIDCAudience)
	if err != nil {
		return fmt.Errorf("oidc: %w", err)
	}
	// Verifier order is fastest-reject-first: worker tokens have a
	// distinctive shape; static tokens are short opaque strings; OIDC
	// id_tokens are JWTs (most expensive to verify). OIDC is appended
	// only when configured — the interceptor's contract is that nil
	// verifiers are skipped, but omitting them entirely is cleaner.
	vset := verifiers.Set{
		&verifiers.WorkerVerifier{HMAC: workerHMAC},
		staticVer,
	}
	if oidcVer != nil {
		vset = append(vset, oidcVer)
	}

	launcher, err := buildLauncher(cfg, logger)
	if err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	defer func() { _ = launcher.Close() }()

	sessionState, sessionCleaner := buildSessionState(cfg, objStore, logger)

	// Chunk-18 metrics. Single PromRecorder shared across services.
	recorder := metrics.NewPromRecorder()

	// Broadcaster: cross-pod run-event fan-out. With the Postgres
	// records backend the API runs as a multi-replica Deployment, so
	// the worker's POST and the UI's stream often land on different
	// pods — the Postgres impl uses LISTEN/NOTIFY against the same
	// records DB to bridge them. SQLite stays single-replica and
	// gets the in-memory impl.
	podID := os.Getenv("HOSTNAME")
	if podID == "" {
		podID = "local-" + uuid.NewString()
	}
	var bcaster broadcaster.Broadcaster
	switch cfg.RecordsBackend {
	case "postgres":
		bp, err := broadcaster.NewPostgres(context.Background(), cfg.PostgresDSN, podID, workerssvc.NewRecordsEventFetcher(recStore))
		if err != nil {
			return fmt.Errorf("broadcaster postgres: %w", err)
		}
		bcaster = bp
		logger.Info("broadcaster backend=postgres", "pod", podID)
	default:
		bcaster = broadcaster.NewInMem()
		logger.Info("broadcaster backend=in-mem", "pod", podID)
	}
	defer func() { _ = bcaster.Close() }()

	workers := workerssvc.NewService(workerssvc.ServiceOptions{
		Records:                        recStore,
		Launcher:                       launcher,
		Broadcaster:                    bcaster,
		WorkerHMAC:                     workerHMAC,
		CallbackURL:                    cfg.WorkerCallback,
		WorkerImage:                    cfg.WorkerImage,
		Logger:                         logger.With("service", "workers"),
		SessionState:                   sessionState,
		SessionStateCleaner:            sessionCleaner,
		MountWorkerServiceAccountToken: cfg.WorkerMountSAToken,
		WorkerExtraEnv:                 cfg.WorkerEnv,
		Metrics:                        recorder,
	})

	// Recount active runs (PENDING + RUNNING) so the active_runs gauge
	// survives a restart. Best-effort — a count failure logs but
	// doesn't gate startup.
	if active, err := workerssvc.CountActiveRuns(bootCtx, recStore); err != nil {
		logger.Warn("active-run recount failed", "err", err)
	} else if active > 0 {
		recorder.RecountActive(active)
		logger.Info("active runs recounted", "count", active)
	}

	adminTokens := map[string]bool{}
	for _, n := range cfg.AuthAdminTokens {
		adminTokens[n] = true
	}
	configService := configsvc.NewService(configsvc.ServiceOptions{
		Records:         recStore,
		Secrets:         configSecrets,
		OAuth:           oauthDispenser,
		StateSigningKey: workerHMAC,
		RunOwnerLookup: func(ctx context.Context, runID string) (string, error) {
			owner, _, err := workerssvc.LookupRunOwner(ctx, recStore, runID)
			return owner, err
		},
		AdminGroup:  cfg.AuthAdminGroup,
		AdminTokens: adminTokens,
		Logger:      logger.With("service", "config"),
	})
	catalog, err := configsvc.LoadCatalog(cfg.ConnectorCatalogFile)
	if err != nil {
		return fmt.Errorf("config catalog: %w", err)
	}
	if created, updated, err := configService.ReconcileCatalog(bootCtx, catalog); err != nil {
		return fmt.Errorf("config catalog reconcile: %w", err)
	} else if created > 0 || updated > 0 {
		logger.Info("connector catalog reconciled", "created", created, "updated", updated)
	}

	appDeployer, err := buildAppDeployer(cfg, launcher, logger)
	if err != nil {
		return fmt.Errorf("app deployer: %w", err)
	}
	defer func() {
		if appDeployer != nil {
			_ = appDeployer.Close()
		}
	}()

	artifactsService := artifactssvc.NewService(artifactssvc.ServiceOptions{
		Records: recStore,
		Objects: objStore,
		Bucket:  cfg.ArtifactsBucket,
		Runs: runOwnerLookupFunc(func(ctx context.Context, runID string) (string, string, error) {
			return workerssvc.LookupRunOwner(ctx, recStore, runID)
		}),
		AppDeployer:    appDeployer,
		AppDownloadTTL: cfg.AppDownloadTTL,
		Verifiers:      vset,
		Metrics:        recorder,
		Logger:         logger.With("service", "artifacts"),
	})

	scheduler, err := buildScheduler(cfg, launcher, logger)
	if err != nil {
		return fmt.Errorf("scheduler: %w", err)
	}
	defer func() {
		if scheduler != nil {
			_ = scheduler.Close()
		}
	}()

	schedulingService := schedulingsvc.NewService(schedulingsvc.ServiceOptions{
		Records:     recStore,
		Scheduler:   scheduler,
		APIURL:      cfg.APIInternalURL,
		FireImage:   cfg.SchedulerFireImage,
		TokenSecret: cfg.SchedulerTokenSecretName,
		Metrics:     recorder,
		Logger:      logger.With("service", "scheduling"),
	})
	if res, err := schedulingService.ReconcileSchedules(bootCtx); err != nil {
		logger.Warn("scheduling reconcile errors", "applied", res.Applied, "reaped", res.Reaped, "err_count", len(res.Errors))
	} else if res.Applied+res.Reaped > 0 {
		logger.Info("schedules reconciled", "applied", res.Applied, "reaped", res.Reaped)
	}

	reg := &server.Registry{
		Storage:    &storagesvc.Service{Records: recStore, Objects: objStore},
		Workers:    workers,
		Config:     configService,
		SecretsAPI: &secretsapisvc.Service{},
		Auth: &authapisvc.Service{
			OAuth:           oauthDispenser,
			StateSigningKey: workerHMAC,
			AdminGroup:      cfg.AuthAdminGroup,
			AdminTokens:     adminTokens,
		},
		Artifacts:  artifactsService,
		Scheduling: schedulingService,
	}

	grpcServer := server.NewGRPC(logger, reg, vset)

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	httpMux := http.NewServeMux()
	gwMux, err := server.NewGateway(context.Background(), grpcListener.Addr().String())
	if err != nil {
		return fmt.Errorf("gateway: %w", err)
	}
	// Mount the artifacts proxy at /v1/ so it sees every /v1/* path
	// and can intercept /v1/artifacts/{id}/view (chunk 16). Mounting
	// it on /v1/artifacts/ (the natural prefix) would trip Go's
	// ServeMux trailing-slash auto-redirect, which breaks
	// ListArtifacts at /v1/artifacts. The handler delegates everything
	// non-view back to gwMux unchanged.
	// User-discovery endpoint backing chunk-19 share-dialog dropdown.
	// Only wired when the OpenBao secrets backend is in use — there's
	// no other directory the chart can list users from. With the file
	// backend, the UI proxy gets a 404 → share dialog falls back to
	// "Everyone's already on the list" (no regression vs pre-feature).
	if identityDir != nil {
		usersService := &users.Service{
			Directory: identityDir,
			Verifiers: vset,
			Logger:    logger.With("service", "users"),
			Group:     cfg.OpenBaoUsersGroup,
		}
		// Mount BEFORE the artifacts proxy so /v1/users wins over the
		// /v1/ catch-all. http.ServeMux longer-prefix-wins handles the
		// ordering deterministically; the registration order doesn't
		// matter for correctness, but we put this first for readability.
		httpMux.Handle("/v1/users", usersService.Handler())
	}
	// Worker-internal session-state HTTP surface, only when the
	// object backend is selected. Chains in front of the artifacts
	// proxy so unrelated /v1/ paths fall through unchanged.
	var v1Handler = artifactsService.ProxyHandler(gwMux)
	if cfg.SessionStateBackend == string(runtime.SessionStateBackendObject) && cfg.SessionStateObjectBucket != "" {
		ss := sessionstatesvc.NewService(sessionstatesvc.ServiceOptions{
			Records:   recStore,
			Objects:   objStore,
			Bucket:    cfg.SessionStateObjectBucket,
			Verifiers: vset,
			Logger:    logger.With("service", "session-state"),
		})
		v1Handler = ss.Handler(v1Handler)
	}
	httpMux.Handle("/v1/", v1Handler)
	httpMux.Handle("/healthz", server.HealthzHandler())
	httpMux.Handle("/readyz", server.HealthzHandler())

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Chunk-18 metrics listener: dedicated port at NS_METRICS_ADDR
	// (default :9090). /metrics for Prometheus scrape, /healthz so
	// kubelet probes can target the same port if operators prefer.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", recorder.Handler())
	metricsMux.Handle("/healthz", server.HealthzHandler())
	metricsSrv := &http.Server{
		Addr:              cfg.MetricsAddr,
		Handler:           metricsMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("starting nightshift-api",
		"grpc", cfg.GRPCAddr,
		"http", cfg.HTTPAddr,
		"metrics", cfg.MetricsAddr,
		"objects_backend", cfg.ObjectsBackend,
		"runtime", cfg.Runtime,
		"worker_callback", cfg.WorkerCallback,
		"auth_oidc", oidcVer.Issuer(),
		"auth_static_tokens", staticVer.Len(),
	)

	lc := &server.Lifecycle{
		Logger:        logger,
		GRPCServer:    grpcServer,
		GRPCListener:  grpcListener,
		HTTPServer:    httpSrv,
		MetricsServer: metricsSrv,
		DrainTimeout:  cfg.DrainTimeout,
	}
	return lc.Run(context.Background())
}

// buildAppDeployer constructs a runtime.AppDeployer based on the
// active runtime mode. For NS_RUNTIME=kubernetes it reuses the
// JobLauncher's clientset; for stub it returns the in-memory stub.
func buildAppDeployer(cfg *Config, launcher runtime.JobLauncher, logger *slog.Logger) (runtime.AppDeployer, error) {
	switch cfg.Runtime {
	case "kubernetes":
		k, ok := launcher.(*runtime.KubernetesLauncher)
		if !ok {
			return nil, fmt.Errorf("app deployer: expected KubernetesLauncher for runtime=kubernetes, got %T", launcher)
		}
		ns := cfg.AppDeployNamespace
		if ns == "" {
			ns = cfg.KubeNamespace
		}
		logger.Info("app deployer", "namespace", ns, "nginx_image", cfg.AppNginxImage, "init_image", cfg.AppInitImage)
		return runtime.NewKubernetesAppDeployer(k.Client, ns, runtime.AppDeployerConfig{
			NginxImage: cfg.AppNginxImage,
			InitImage:  cfg.AppInitImage,
		})
	case "stub":
		logger.Info("app deployer=stub (no real K8s backend; /view will return 503)")
		return runtime.NewStubAppDeployer(), nil
	default:
		return nil, fmt.Errorf("app deployer: unknown runtime %q", cfg.Runtime)
	}
}

// buildScheduler constructs a runtime.Scheduler. K8s when the runtime
// mode is kubernetes (extracts the launcher's clientset), stub
// otherwise. Mirrors buildAppDeployer (chunk 16).
func buildScheduler(cfg *Config, launcher runtime.JobLauncher, logger *slog.Logger) (runtime.Scheduler, error) {
	switch cfg.Runtime {
	case "kubernetes":
		k, ok := launcher.(*runtime.KubernetesLauncher)
		if !ok {
			return nil, fmt.Errorf("scheduler: expected KubernetesLauncher for runtime=kubernetes, got %T", launcher)
		}
		ns := cfg.AppDeployNamespace
		if ns == "" {
			ns = cfg.KubeNamespace
		}
		logger.Info("scheduler", "namespace", ns, "fire_image", cfg.SchedulerFireImage, "token_secret", cfg.SchedulerTokenSecretName)
		return runtime.NewKubernetesScheduler(k.Client, ns)
	case "stub":
		logger.Info("scheduler=stub (no real K8s backend; schedules persist but never fire)")
		return runtime.NewStubScheduler(), nil
	default:
		return nil, fmt.Errorf("scheduler: unknown runtime %q", cfg.Runtime)
	}
}

// runOwnerLookupFunc adapts a plain func into the artifacts.RunLookup
// interface so the Artifacts service can derive the owner of an
// in-flight run from a worker credential's bound run_id without
// importing internal/api/workers (which would create a cycle).
type runOwnerLookupFunc func(ctx context.Context, runID string) (string, string, error)

func (f runOwnerLookupFunc) LookupRunOwner(ctx context.Context, runID string) (string, string, error) {
	return f(ctx, runID)
}

// loadWorkerHMAC reads the shared HMAC key used to sign worker
// credentials. v1 keeps the key small and operator-supplied;
// chunk 12 will source it from OpenBao via the same Secrets
// interface.
func loadWorkerHMAC(s secrets.Secrets) ([]byte, error) {
	kv, err := s.Get(context.Background(), workerHMACPath)
	if err != nil {
		return nil, fmt.Errorf("lookup %s: %w", workerHMACPath, err)
	}
	val := kv["value"]
	if val == "" {
		val = kv["secret"]
	}
	if val == "" {
		return nil, errors.New("worker hmac path present but neither 'value' nor 'secret' field set")
	}
	if len(val) < 16 {
		return nil, fmt.Errorf("worker hmac must be ≥ 16 bytes, got %d", len(val))
	}
	return []byte(val), nil
}

// buildConfigSecrets selects the Secrets KV backend, the OAuth
// dispenser, and the identity directory used by the Config + Auth
// services and the user-discovery handler. file (default) is
// read-only and reuses the bootstrap fileSecrets so per-user
// connector token writes return Unimplemented; the file backend has
// no OAuthDispenser or Directory, so those returns are nil and the
// dependent surfaces (OAuth flows in Config + Auth, /v1/users) all
// degrade to Unimplemented / 404.
//
// openbao instantiates two independent client structs against the
// same OpenBao deployment: secrets.OpenBao for the KV-v2 plugin and
// identity.OpenBao for the identity engine. They share the operator's
// SA token + auth role but maintain separate cached client_tokens.
// The OAuth dispenser is built on top of secrets — Native consumes
// the secrets.Secrets interface — so adding a future K8s/Vault secrets
// backend swaps the storage substrate without touching OAuth code.
func buildConfigSecrets(cfg *Config, fileSecrets *secrets.File, logger *slog.Logger) (secrets.Secrets, oauth.OAuthDispenser, identity.Directory, error) {
	switch cfg.SecretsBackend {
	case "", "file":
		logger.Info("config secrets backend=file (per-user connector tokens read-only; OAuth dispenser + identity directory disabled)")
		return fileSecrets, nil, nil, nil
	case "openbao":
		logger.Info("config secrets backend=openbao",
			"addr", cfg.OpenBaoAddr,
			"role", cfg.OpenBaoAuthRole,
			"mount", cfg.OpenBaoKVMount,
		)
		if cfg.OpenBaoAddr == "" {
			return nil, nil, nil, fmt.Errorf("NS_OPENBAO_ADDR required when NS_SECRETS_BACKEND=openbao")
		}
		kv, err := secrets.NewOpenBao(secrets.OpenBaoConfig{
			Addr:     cfg.OpenBaoAddr,
			AuthRole: cfg.OpenBaoAuthRole,
			KVMount:  cfg.OpenBaoKVMount,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		// OAuth dispenser is always Native (in-process PKCE/S256
		// client). Pluggability lives at the secrets layer above:
		// Native uses kv as its sole storage substrate, and any
		// secrets.Secrets impl (OpenBao today, future K8s/Vault) works
		// transparently. The API consumes only oauth.OAuthDispenser
		// and is unaware of which secrets backend stores tokens.
		dispenser := oauth.NewNative(kv)
		dir, err := identity.NewOpenBao(identity.OpenBaoConfig{
			Addr:     cfg.OpenBaoAddr,
			AuthRole: cfg.OpenBaoAuthRole,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		return kv, dispenser, dir, nil
	}
	return nil, nil, nil, fmt.Errorf("unknown NS_SECRETS_BACKEND=%q (expected file|openbao)", cfg.SecretsBackend)
}

// buildSessionState resolves the chunk-13 per-session volume + cascade
// from env config. Backends that have no in-process consumer (object
// without a worker round-trip; chunk 14) still return a working
// cleaner so DeleteSession's cascade applies.
func buildSessionState(cfg *Config, objStore objects.ObjectStore, logger *slog.Logger) (runtime.SessionStateConfig, runtime.SessionCleaner) {
	state := runtime.SessionStateConfig{
		Backend:      runtime.SessionStateBackend(cfg.SessionStateBackend),
		MountPath:    cfg.SessionStateMountPath,
		PVCName:      cfg.SessionStatePVCName,
		HostRoot:     cfg.SessionStateHostRoot,
		ObjectBucket: cfg.SessionStateObjectBucket,
	}
	logger.Info("session-state",
		"backend", state.Backend,
		"mount", state.MountPath,
		"pvc", state.PVCName,
		"host_root", state.HostRoot,
		"object_bucket", state.ObjectBucket,
	)
	switch state.Backend {
	case runtime.SessionStateBackendPVC:
		return state, &runtime.LocalFSCleaner{Root: state.MountPath}
	case runtime.SessionStateBackendHost:
		return state, &runtime.LocalFSCleaner{Root: state.HostRoot}
	case runtime.SessionStateBackendObject:
		return state, &runtime.ObjectCleaner{Store: objStore, Bucket: state.ObjectBucket}
	}
	return state, runtime.NoopCleaner{}
}

func buildLauncher(cfg *Config, logger *slog.Logger) (runtime.JobLauncher, error) {
	switch cfg.Runtime {
	case "stub":
		logger.Info("runtime=stub", "worker_binary", cfg.WorkerBinary)
		return runtime.NewStubLauncher(cfg.WorkerBinary)
	case "kubernetes":
		logger.Info("runtime=kubernetes",
			"namespace", cfg.KubeNamespace,
			"service_account", cfg.KubeWorkerSA,
			"worker_image", cfg.WorkerImage,
		)
		return runtime.NewKubernetesLauncher(cfg.KubeconfigPath, cfg.KubeNamespace, cfg.KubeWorkerSA)
	default:
		return nil, fmt.Errorf("unknown NS_RUNTIME=%q", cfg.Runtime)
	}
}
