// Package artifacts implements nightshift.v1.Artifacts.
//
// The service layers on internal/records (Artifact + ArtifactPermission
// metadata) and internal/objects (blob bytes + companion preview).
// cr0n-a parity is the design contract — see artifact_routes.py and
// internal_routes.py for the behavioral source of truth.
//
// Chunk 15 ships object artifacts. App artifacts (CreateAppArtifact,
// DEPLOYMENT_STATE_*, hosted workload, app preview proxy) stay
// Unimplemented and land in chunk 16.
//
// Lifecycle events (`artifact.created` / `artifact.updated`) are NOT
// emitted: cr0n-a never wired them despite SPEC.md mentioning them
// (store.create_artifact / store.update_artifact write the row and
// return; UI infers artifact lifecycle from the worker's `tool_use`
// event and a follow-up ListArtifacts poll). Chunk 15 matches that
// behavior.
package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/metrics"
	"github.com/nightshiftco/nightshift/internal/objects"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// RunLookup resolves the owning user_id for a run. Implemented at
// wiring time over workers.LookupRunOwner; declared here so this
// package doesn't import internal/api/workers.
type RunLookup interface {
	LookupRunOwner(ctx context.Context, runID string) (userID, sessionID string, err error)
}

// AppDeployer is re-exported from internal/runtime so callers can
// satisfy this option without naming the runtime package directly.
type AppDeployer = runtime.AppDeployer

// ServiceOptions configures a Service. Records, Objects, and Bucket
// are required.
type ServiceOptions struct {
	Records     records.RecordStore
	Objects     objects.ObjectStore
	Bucket      string
	Runs        RunLookup
	Logger      *slog.Logger
	DownloadTTL time.Duration

	// AppDeployer materializes app artifacts (Create/Update/Delete K8s
	// Deployment + Service). Optional — when nil, CreateAppArtifact +
	// app-update + app-delete return Unimplemented. Wired in main.go
	// from internal/runtime via a small adapter.
	AppDeployer AppDeployer

	// AppDownloadTTL is the TTL on the presigned URL the init container
	// fetches the HTML with. Default 30m — generous for image pull +
	// pod scheduling delays.
	AppDownloadTTL time.Duration

	// Verifiers is used by the proxy handler (HTTP, non-gateway) to
	// authenticate Bearer-credentialed callers. Optional — when nil,
	// the proxy handler refuses all private app fetches with 401.
	Verifiers verifiers.Set

	// Metrics is the chunk-18 recorder. Optional — nil-safe via
	// metrics.Get; existing tests pass nil and stay valid.
	Metrics metrics.Recorder

	// Test seams.
	NewID func() string
	Now   func() time.Time
}

// Service is the nightshift.v1.ArtifactsServer implementation.
type Service struct {
	nsv1.UnimplementedArtifactsServer

	records        records.RecordStore
	objects        objects.ObjectStore
	bucket         string
	runs           RunLookup
	logger         *slog.Logger
	downloadTTL    time.Duration
	appDeployer    AppDeployer
	appDownloadTTL time.Duration
	verifiers      verifiers.Set
	metrics        metrics.Recorder

	newID func() string
	now   func() time.Time
}

// NewService constructs a Service. Misconfiguration panics — wire-up
// errors should fail loudly at startup.
func NewService(opts ServiceOptions) *Service {
	if opts.Records == nil {
		panic("artifacts.NewService: Records required")
	}
	if opts.Objects == nil {
		panic("artifacts.NewService: Objects required")
	}
	if opts.Bucket == "" {
		panic("artifacts.NewService: Bucket required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	newID := opts.NewID
	if newID == nil {
		newID = func() string { return uuid.NewString() }
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	ttl := opts.DownloadTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	appTTL := opts.AppDownloadTTL
	if appTTL <= 0 {
		appTTL = 30 * time.Minute
	}
	return &Service{
		records:        opts.Records,
		objects:        opts.Objects,
		bucket:         opts.Bucket,
		runs:           opts.Runs,
		logger:         logger,
		downloadTTL:    ttl,
		appDeployer:    opts.AppDeployer,
		appDownloadTTL: appTTL,
		verifiers:      opts.Verifiers,
		metrics:        metrics.Get(opts.Metrics),
		newID:          newID,
		now:            now,
	}
}

// -----------------------------------------------------------------------------
// CreateObjectArtifact
// -----------------------------------------------------------------------------

func (s *Service) CreateObjectArtifact(ctx context.Context, req *nsv1.CreateObjectArtifactRequest) (*nsv1.CreateObjectArtifactResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	if len(req.GetContent()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "content required")
	}
	if req.GetContentType() == "" {
		return nil, status.Error(codes.InvalidArgument, "content_type required")
	}

	ownerID, runID, sessionID, err := s.deriveOwnerRunSession(ctx, req.GetOwnerId(), req.GetRunId(), req.GetSessionId())
	if err != nil {
		return nil, err
	}

	// Idempotency replay: if a prior CreateObjectArtifact was made by
	// the same caller with the same key, return that artifact when the
	// content matches; INVALID_ARGUMENT when it doesn't (artifacts.md
	// §9). The composite "owner + key" scope matches cr0n's
	// idempotency model; collisions across owners are not collisions.
	if k := req.GetIdempotencyKey(); k != "" {
		existing, priorHash, hit, err := s.findByIdemKey(ctx, ownerID, k)
		if err != nil {
			return nil, err
		}
		if hit {
			if priorHash != "" && priorHash != contentHash(req.GetContent()) {
				return nil, status.Error(codes.InvalidArgument, "idempotency_key conflict: prior request had different content")
			}
			return &nsv1.CreateObjectArtifactResponse{Artifact: existing}, nil
		}
	}

	id := "art_" + s.newID()
	now := s.now()
	objKey := objectKey(id, req.GetName())

	obj, err := s.objects.PutBytes(ctx, s.bucket, objKey, req.GetContentType(), req.GetContent())
	if err != nil {
		return nil, objectErr(err)
	}

	hasPreview := false
	if len(req.GetPreviewHtml()) > 0 {
		if _, err := s.objects.PutBytes(ctx, s.bucket, previewKey(id), "text/html; charset=utf-8", req.GetPreviewHtml()); err != nil {
			// Roll back the blob to avoid an orphan and return.
			_ = s.objects.Delete(ctx, s.bucket, objKey)
			return nil, objectErr(err)
		}
		hasPreview = true
	}

	art := &nsv1.Artifact{
		Id:           id,
		Type:         nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT,
		Name:         req.GetName(),
		Description:  req.GetDescription(),
		OwnerId:      ownerID,
		RunId:        runID,
		SessionId:    sessionID,
		Public:       req.GetPublic(),
		ObjectBucket: s.bucket,
		ObjectKey:    objKey,
		ContentType:  req.GetContentType(),
		SizeBytes:    obj.SizeBytes,
		HasPreview:   hasPreview,
		CreatedAt:    timestamppb.New(now),
		UpdatedAt:    timestamppb.New(now),
	}

	rec, err := artifactToRecord(art, req.GetIdempotencyKey(), contentHash(req.GetContent()))
	if err != nil {
		_ = s.objects.Delete(ctx, s.bucket, objKey)
		if hasPreview {
			_ = s.objects.Delete(ctx, s.bucket, previewKey(id))
		}
		s.metrics.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT, "error")
		return nil, status.Errorf(codes.Internal, "build artifact record: %v", err)
	}
	zero := int64(0)
	if _, err := s.records.Put(ctx, rec, &zero, req.GetIdempotencyKey()); err != nil {
		_ = s.objects.Delete(ctx, s.bucket, objKey)
		if hasPreview {
			_ = s.objects.Delete(ctx, s.bucket, previewKey(id))
		}
		s.metrics.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT, "error")
		return nil, recordErr(err)
	}
	s.metrics.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT, "success")
	return &nsv1.CreateObjectArtifactResponse{Artifact: art}, nil
}

// -----------------------------------------------------------------------------
// CreateAppArtifact
// -----------------------------------------------------------------------------

func (s *Service) CreateAppArtifact(ctx context.Context, req *nsv1.CreateAppArtifactRequest) (*nsv1.CreateAppArtifactResponse, error) {
	if s.appDeployer == nil {
		return nil, status.Error(codes.Unimplemented, "app artifacts require an AppDeployer; install with NS_RUNTIME=kubernetes")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	if req.GetHtmlContent() == "" {
		return nil, status.Error(codes.InvalidArgument, "html_content required")
	}

	ownerID, runID, _, err := s.deriveOwnerRunSession(ctx, req.GetOwnerId(), req.GetRunId(), "")
	if err != nil {
		return nil, err
	}

	// Idempotency replay: same composite owner+key returns the prior
	// artifact when content matches, INVALID_ARGUMENT otherwise (mirrors
	// CreateObjectArtifact). Hash is computed over the HTML bytes.
	if k := req.GetIdempotencyKey(); k != "" {
		existing, priorHash, hit, err := s.findByIdemKey(ctx, ownerID, k)
		if err != nil {
			return nil, err
		}
		if hit {
			if priorHash != "" && priorHash != contentHash([]byte(req.GetHtmlContent())) {
				return nil, status.Error(codes.InvalidArgument, "idempotency_key conflict: prior request had different content")
			}
			return &nsv1.CreateAppArtifactResponse{Artifact: existing}, nil
		}
	}

	id := "art_" + s.newID()
	now := s.now()
	objKey := appObjectKey(id)
	htmlBytes := []byte(req.GetHtmlContent())

	// 1. Upload the HTML to Object storage. Same backend the chunk-15
	// object artifacts use; same bucket. The init container will fetch
	// from a presigned URL we mint next.
	if _, err := s.objects.PutBytes(ctx, s.bucket, objKey, "text/html; charset=utf-8", htmlBytes); err != nil {
		return nil, objectErr(err)
	}

	// 2. Mint a long-TTL presigned URL for the init container to fetch.
	dlURL, _, err := s.objects.DownloadURL(ctx, s.bucket, objKey, s.appDownloadTTL)
	if err != nil {
		_ = s.objects.Delete(ctx, s.bucket, objKey)
		return nil, objectErr(err)
	}

	// 3. Deploy K8s resources via the AppDeployer.
	depRes, err := s.appDeployer.Deploy(ctx, runtime.AppSpec{
		ArtifactID:  id,
		Name:        req.GetName(),
		DownloadURL: dlURL,
	})
	if err != nil {
		_ = s.objects.Delete(ctx, s.bucket, objKey)
		s.metrics.AppDeployed("error")
		s.metrics.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_APP, "error")
		return nil, status.Errorf(codes.Internal, "deploy app: %v", err)
	}
	s.metrics.AppDeployed("success")

	// 4. Persist the artifact record.
	art := &nsv1.Artifact{
		Id:           id,
		Type:         nsv1.ArtifactType_ARTIFACT_TYPE_APP,
		Name:         req.GetName(),
		Description:  req.GetDescription(),
		OwnerId:      ownerID,
		RunId:        runID,
		Public:       req.GetPublic(),
		ObjectBucket: s.bucket,
		ObjectKey:    objKey,
		AppUrl:       depRes.ServiceURL,
		AppStatus:    nsv1.DeploymentState_DEPLOYMENT_STATE_READY,
		CreatedAt:    timestamppb.New(now),
		UpdatedAt:    timestamppb.New(now),
	}
	rec, err := artifactToRecord(art, req.GetIdempotencyKey(), contentHash(htmlBytes))
	if err != nil {
		_ = s.appDeployer.Delete(ctx, id)
		_ = s.objects.Delete(ctx, s.bucket, objKey)
		s.metrics.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_APP, "error")
		return nil, status.Errorf(codes.Internal, "build artifact record: %v", err)
	}
	zero := int64(0)
	if _, err := s.records.Put(ctx, rec, &zero, req.GetIdempotencyKey()); err != nil {
		_ = s.appDeployer.Delete(ctx, id)
		_ = s.objects.Delete(ctx, s.bucket, objKey)
		s.metrics.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_APP, "error")
		return nil, recordErr(err)
	}
	s.metrics.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_APP, "success")
	return &nsv1.CreateAppArtifactResponse{Artifact: art}, nil
}

// -----------------------------------------------------------------------------
// GetArtifact
// -----------------------------------------------------------------------------

func (s *Service) GetArtifact(ctx context.Context, req *nsv1.GetArtifactRequest) (*nsv1.GetArtifactResponse, error) {
	if req.GetArtifactId() == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id required")
	}
	art, err := s.loadArtifact(ctx, req.GetArtifactId())
	if err != nil {
		return nil, err
	}
	viewerID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	grants, err := s.listGrants(ctx, art.GetId())
	if err != nil {
		return nil, err
	}
	p := verifiers.FromContext(ctx)
	if !canRead(p, art, grants, viewerID) {
		return nil, notFoundForRead()
	}
	return &nsv1.GetArtifactResponse{Artifact: art}, nil
}

// -----------------------------------------------------------------------------
// ListArtifacts
// -----------------------------------------------------------------------------

func (s *Service) ListArtifacts(ctx context.Context, req *nsv1.ListArtifactsRequest) (*nsv1.ListArtifactsResponse, error) {
	viewerID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}

	// A caller-supplied owner_id filter that names a different user
	// collapses to empty — outer surface MUST NOT leak other users'
	// artifacts (artifacts.md §3 collapse rule). owner_id == viewerID
	// and the empty default both mean "what the viewer can see".
	if w := req.GetOwnerId(); w != "" && w != viewerID {
		return &nsv1.ListArtifactsResponse{}, nil
	}

	owned, err := s.listAllOwned(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	shared, err := s.listAllSharedWithViewer(ctx, viewerID)
	if err != nil {
		return nil, err
	}

	// Merge owned ∪ shared; dedupe by id (a grant to the owner is
	// rejected at share time so collisions shouldn't happen, but be
	// defensive against legacy rows).
	seen := make(map[string]struct{}, len(owned)+len(shared))
	merged := make([]*nsv1.Artifact, 0, len(owned)+len(shared))
	for _, a := range owned {
		seen[a.GetId()] = struct{}{}
		merged = append(merged, a)
	}
	for _, a := range shared {
		if _, dup := seen[a.GetId()]; dup {
			continue
		}
		seen[a.GetId()] = struct{}{}
		merged = append(merged, a)
	}

	typeF := req.GetTypeFilter()
	runF := req.GetRunId()
	sessF := req.GetSessionId()
	filtered := merged[:0]
	for _, a := range merged {
		if typeF != nsv1.ArtifactType_ARTIFACT_TYPE_UNSPECIFIED && a.GetType() != typeF {
			continue
		}
		if runF != "" && a.GetRunId() != runF {
			continue
		}
		if sessF != "" && a.GetSessionId() != sessF {
			continue
		}
		filtered = append(filtered, a)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		ti := filtered[i].GetUpdatedAt().AsTime()
		tj := filtered[j].GetUpdatedAt().AsTime()
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return filtered[i].GetId() < filtered[j].GetId()
	})

	// In-memory offset paging. The merged result spans two collections,
	// so records.List's (updated_at, key) cursor format doesn't apply.
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = 100
	}
	start, err := decodeOffsetToken(req.GetPageToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page_token")
	}
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	next := ""
	if end < len(filtered) {
		next = encodeOffsetToken(end)
	} else {
		end = len(filtered)
	}
	return &nsv1.ListArtifactsResponse{
		Artifacts:     filtered[start:end],
		NextPageToken: next,
	}, nil
}

// listAllOwned drains every artifact whose owner_id == viewerID.
// records.List caps a single page at 500, so we iterate to keep
// downstream merge/sort correct for users with larger libraries.
func (s *Service) listAllOwned(ctx context.Context, viewerID string) ([]*nsv1.Artifact, error) {
	var (
		out   []*nsv1.Artifact
		token string
	)
	for {
		page, next, err := s.records.List(ctx, records.ListQuery{
			Collection:       artifactsCollection,
			AttributeFilters: map[string]string{attrOwnerID: viewerID},
			PageSize:         500,
			PageToken:        token,
		})
		if err != nil {
			return nil, recordErr(err)
		}
		for _, rec := range page {
			art, err := recordToArtifact(rec)
			if err != nil {
				s.logger.Warn("artifacts: skipping malformed record", "key", rec.Key, "err", err)
				continue
			}
			out = append(out, art)
		}
		if next == "" {
			return out, nil
		}
		token = next
	}
}

// listAllSharedWithViewer drains permission grants where user_id ==
// viewerID and loads each referenced artifact. Grants pointing at a
// missing artifact (deleted but cascade lost a race, or test fixtures)
// are silently dropped — listing is a soft surface.
func (s *Service) listAllSharedWithViewer(ctx context.Context, viewerID string) ([]*nsv1.Artifact, error) {
	var (
		out   []*nsv1.Artifact
		token string
	)
	for {
		page, next, err := s.records.List(ctx, records.ListQuery{
			Collection:       permissionsCollection,
			AttributeFilters: map[string]string{attrPermUserID: viewerID},
			PageSize:         500,
			PageToken:        token,
		})
		if err != nil {
			return nil, recordErr(err)
		}
		for _, rec := range page {
			perm, err := decodePermission(rec)
			if err != nil {
				s.logger.Warn("artifacts: skipping malformed permission record", "key", rec.Key, "err", err)
				continue
			}
			if perm.GetRole() == nsv1.ArtifactRole_ARTIFACT_ROLE_UNSPECIFIED {
				continue
			}
			art, err := s.loadArtifact(ctx, perm.GetArtifactId())
			if err != nil {
				if status.Code(err) == codes.NotFound {
					continue
				}
				return nil, err
			}
			out = append(out, art)
		}
		if next == "" {
			return out, nil
		}
		token = next
	}
}

// encodeOffsetToken / decodeOffsetToken implement the opaque page token
// used by ListArtifacts. The format is `o|<n>` hex-encoded; the prefix
// rejects stray inputs (a misrouted records.List cursor would decode
// successfully but lack the prefix).
func encodeOffsetToken(n int) string {
	return hex.EncodeToString([]byte("o|" + strconv.Itoa(n)))
}

func decodeOffsetToken(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return 0, err
	}
	raw := string(b)
	if len(raw) < 3 || raw[:2] != "o|" {
		return 0, fmt.Errorf("invalid page_token")
	}
	n, err := strconv.Atoi(raw[2:])
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid page_token offset")
	}
	return n, nil
}

// -----------------------------------------------------------------------------
// UpdateArtifact
// -----------------------------------------------------------------------------

func (s *Service) UpdateArtifact(ctx context.Context, req *nsv1.UpdateArtifactRequest) (*nsv1.UpdateArtifactResponse, error) {
	if req.GetArtifactId() == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id required")
	}
	rec, err := s.records.Get(ctx, artifactsCollection, req.GetArtifactId())
	if err != nil {
		if errors.Is(err, records.ErrNotFound) {
			return nil, notFoundForRead()
		}
		return nil, recordErr(err)
	}
	art, err := recordToArtifact(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decode artifact: %v", err)
	}
	viewerID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	grants, err := s.listGrants(ctx, art.GetId())
	if err != nil {
		return nil, err
	}
	if !canEdit(art, grants, viewerID) {
		// Hide the artifact entirely from non-readers; return
		// PERMISSION_DENIED to readers who lack edit.
		p := verifiers.FromContext(ctx)
		if !canRead(p, art, grants, viewerID) {
			return nil, notFoundForRead()
		}
		return nil, status.Error(codes.PermissionDenied, "edit role required")
	}

	now := s.now()
	dirty := false
	if req.Name != nil {
		art.Name = *req.Name
		dirty = true
	}
	if req.Description != nil {
		art.Description = *req.Description
		dirty = true
	}
	if req.Public != nil {
		if !canAdmin(art, viewerID) {
			return nil, status.Error(codes.PermissionDenied, "owner only: visibility toggle")
		}
		art.Public = *req.Public
		dirty = true
	}

	// Object content replacement.
	if len(req.GetContentBytes()) > 0 {
		if art.GetType() != nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT {
			return nil, status.Error(codes.InvalidArgument, "content_bytes only valid on object artifacts")
		}
		ct := req.GetContentType()
		if ct == "" {
			ct = art.GetContentType()
		}
		if ct == "" {
			return nil, status.Error(codes.InvalidArgument, "content_type required when replacing content_bytes")
		}
		obj, err := s.objects.PutBytes(ctx, art.GetObjectBucket(), art.GetObjectKey(), ct, req.GetContentBytes())
		if err != nil {
			return nil, objectErr(err)
		}
		art.ContentType = ct
		art.SizeBytes = obj.SizeBytes
		dirty = true
	}

	// App HTML replacement: re-upload + trigger rolling restart.
	if req.GetHtmlContent() != "" {
		if art.GetType() != nsv1.ArtifactType_ARTIFACT_TYPE_APP {
			return nil, status.Error(codes.InvalidArgument, "html_content only valid on app artifacts")
		}
		if s.appDeployer == nil {
			return nil, status.Error(codes.Unimplemented, "app artifacts require an AppDeployer")
		}
		htmlBytes := []byte(req.GetHtmlContent())
		if _, err := s.objects.PutBytes(ctx, art.GetObjectBucket(), art.GetObjectKey(), "text/html; charset=utf-8", htmlBytes); err != nil {
			return nil, objectErr(err)
		}
		dlURL, _, err := s.objects.DownloadURL(ctx, art.GetObjectBucket(), art.GetObjectKey(), s.appDownloadTTL)
		if err != nil {
			return nil, objectErr(err)
		}
		if _, err := s.appDeployer.Update(ctx, runtime.AppSpec{
			ArtifactID:  art.GetId(),
			Name:        art.GetName(),
			DownloadURL: dlURL,
		}); err != nil {
			return nil, status.Errorf(codes.Internal, "update app: %v", err)
		}
		dirty = true
	}

	// Preview replacement / clear.
	if req.GetSetPreviewHtml() {
		if art.GetType() != nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT {
			return nil, status.Error(codes.InvalidArgument, "preview only valid on object artifacts")
		}
		if len(req.GetPreviewHtml()) == 0 {
			if art.GetHasPreview() {
				if err := s.objects.Delete(ctx, art.GetObjectBucket(), previewKey(art.GetId())); err != nil && !errors.Is(err, objects.ErrNotFound) {
					return nil, objectErr(err)
				}
			}
			art.HasPreview = false
		} else {
			if _, err := s.objects.PutBytes(ctx, art.GetObjectBucket(), previewKey(art.GetId()), "text/html; charset=utf-8", req.GetPreviewHtml()); err != nil {
				return nil, objectErr(err)
			}
			art.HasPreview = true
		}
		dirty = true
	}

	if !dirty {
		return &nsv1.UpdateArtifactResponse{Artifact: art}, nil
	}
	art.UpdatedAt = timestamppb.New(now)

	idemHash := ""
	if v := rec.Attributes[attrIdemKey]; v != "" {
		// Preserve the original idempotency-key sidecar across updates.
		// It applies to the original content, not subsequent rewrites.
		idemHash = idemHashFromAttr(rec.Attributes)
	}
	updated, err := artifactToRecord(art, rec.Attributes[attrIdemKey], idemHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build artifact record: %v", err)
	}
	updated.Version = rec.Version
	if _, err := s.records.Put(ctx, updated, &rec.Version, ""); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.UpdateArtifactResponse{Artifact: art}, nil
}

// -----------------------------------------------------------------------------
// DeleteArtifact
// -----------------------------------------------------------------------------

func (s *Service) DeleteArtifact(ctx context.Context, req *nsv1.DeleteArtifactRequest) (*nsv1.DeleteArtifactResponse, error) {
	if req.GetArtifactId() == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id required")
	}
	art, err := s.loadArtifact(ctx, req.GetArtifactId())
	if err != nil {
		return nil, err
	}
	viewerID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	if !canAdmin(art, viewerID) {
		// Hide from non-readers.
		grants, lerr := s.listGrants(ctx, art.GetId())
		if lerr != nil {
			return nil, lerr
		}
		p := verifiers.FromContext(ctx)
		if !canRead(p, art, grants, viewerID) {
			return nil, notFoundForRead()
		}
		return nil, status.Error(codes.PermissionDenied, "owner only: delete")
	}

	// App teardown: K8s resources MUST come down BEFORE the Object is
	// deleted, since the next pod restart's init container would 404
	// trying to fetch the (now-missing) HTML.
	if art.GetType() == nsv1.ArtifactType_ARTIFACT_TYPE_APP && s.appDeployer != nil {
		if err := s.appDeployer.Delete(ctx, art.GetId()); err != nil {
			return nil, status.Errorf(codes.Internal, "delete app deployment: %v", err)
		}
	}

	// Best-effort blob + preview cleanup; tolerate NotFound.
	if art.GetObjectKey() != "" {
		if err := s.objects.Delete(ctx, art.GetObjectBucket(), art.GetObjectKey()); err != nil && !errors.Is(err, objects.ErrNotFound) {
			return nil, objectErr(err)
		}
	}
	if art.GetHasPreview() {
		if err := s.objects.Delete(ctx, art.GetObjectBucket(), previewKey(art.GetId())); err != nil && !errors.Is(err, objects.ErrNotFound) {
			return nil, objectErr(err)
		}
	}

	// Cascade permission grants.
	grants, err := s.listGrants(ctx, art.GetId())
	if err != nil {
		return nil, err
	}
	for _, g := range grants {
		if err := s.records.Delete(ctx, permissionsCollection, permissionKey(art.GetId(), g.GetUserId()), nil); err != nil && !errors.Is(err, records.ErrNotFound) {
			return nil, recordErr(err)
		}
	}

	if err := s.records.Delete(ctx, artifactsCollection, art.GetId(), nil); err != nil {
		return nil, recordErr(err)
	}
	s.metrics.ArtifactDeleted(art.GetType())
	return &nsv1.DeleteArtifactResponse{}, nil
}

// -----------------------------------------------------------------------------
// GetArtifactDownloadURL / GetArtifactPreviewURL
// -----------------------------------------------------------------------------

func (s *Service) GetArtifactDownloadURL(ctx context.Context, req *nsv1.GetArtifactDownloadURLRequest) (*nsv1.GetArtifactDownloadURLResponse, error) {
	if req.GetArtifactId() == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id required")
	}
	art, err := s.loadArtifactForRead(ctx, req.GetArtifactId())
	if err != nil {
		return nil, err
	}
	if art.GetType() != nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT {
		return nil, status.Error(codes.FailedPrecondition, "download_url only valid on object artifacts")
	}
	url, exp, err := s.objects.DownloadURL(ctx, art.GetObjectBucket(), art.GetObjectKey(), s.downloadTTL)
	if err != nil {
		return nil, objectErr(err)
	}
	return &nsv1.GetArtifactDownloadURLResponse{
		DownloadUrl: url,
		ExpiresAt:   timestamppb.New(exp),
	}, nil
}

func (s *Service) GetArtifactPreviewURL(ctx context.Context, req *nsv1.GetArtifactPreviewURLRequest) (*nsv1.GetArtifactPreviewURLResponse, error) {
	if req.GetArtifactId() == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id required")
	}
	art, err := s.loadArtifactForRead(ctx, req.GetArtifactId())
	if err != nil {
		return nil, err
	}
	if !art.GetHasPreview() {
		return nil, status.Error(codes.NotFound, "no preview")
	}
	url, exp, err := s.objects.DownloadURL(ctx, art.GetObjectBucket(), previewKey(art.GetId()), s.downloadTTL)
	if err != nil {
		return nil, objectErr(err)
	}
	return &nsv1.GetArtifactPreviewURLResponse{
		PreviewUrl: url,
		ExpiresAt:  timestamppb.New(exp),
	}, nil
}

// -----------------------------------------------------------------------------
// ShareArtifact / RevokeArtifactShare / ListArtifactPermissions
// -----------------------------------------------------------------------------

func (s *Service) ShareArtifact(ctx context.Context, req *nsv1.ShareArtifactRequest) (*nsv1.ShareArtifactResponse, error) {
	if req.GetArtifactId() == "" || req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id and user_id required")
	}
	switch req.GetRole() {
	case nsv1.ArtifactRole_ARTIFACT_ROLE_VIEWER, nsv1.ArtifactRole_ARTIFACT_ROLE_EDITOR:
		// ok
	case nsv1.ArtifactRole_ARTIFACT_ROLE_OWNER:
		return nil, status.Error(codes.InvalidArgument, "ARTIFACT_ROLE_OWNER not grantable in v1")
	default:
		return nil, status.Error(codes.InvalidArgument, "role required")
	}

	art, err := s.loadArtifact(ctx, req.GetArtifactId())
	if err != nil {
		return nil, err
	}
	viewerID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	if !canAdmin(art, viewerID) {
		// Same hide-as-NOT_FOUND treatment as DeleteArtifact.
		grants, lerr := s.listGrants(ctx, art.GetId())
		if lerr != nil {
			return nil, lerr
		}
		p := verifiers.FromContext(ctx)
		if !canRead(p, art, grants, viewerID) {
			return nil, notFoundForRead()
		}
		return nil, status.Error(codes.PermissionDenied, "owner only: share")
	}
	if req.GetUserId() == art.GetOwnerId() {
		return nil, status.Error(codes.InvalidArgument, "cannot share with owner")
	}

	now := s.now()
	perm := &nsv1.ArtifactPermission{
		Id:         "perm_" + s.newID(),
		ArtifactId: art.GetId(),
		UserId:     req.GetUserId(),
		Role:       req.GetRole(),
		GrantedAt:  timestamppb.New(now),
	}
	rec, err := permissionToRecord(perm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build permission record: %v", err)
	}
	// Upsert: if a prior grant exists, preserve its id but replace the
	// role + granted_at. cr0n's INSERT-OR-REPLACE semantics.
	prior, err := s.records.Get(ctx, permissionsCollection, permissionKey(art.GetId(), req.GetUserId()))
	switch {
	case err == nil:
		if existing, derr := decodePermission(prior); derr == nil {
			perm.Id = existing.GetId()
			rec, err = permissionToRecord(perm)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "build permission record: %v", err)
			}
		}
		rec.Version = prior.Version
		if _, err := s.records.Put(ctx, rec, &prior.Version, ""); err != nil {
			return nil, recordErr(err)
		}
	case errors.Is(err, records.ErrNotFound):
		zero := int64(0)
		if _, err := s.records.Put(ctx, rec, &zero, ""); err != nil {
			return nil, recordErr(err)
		}
	default:
		return nil, recordErr(err)
	}
	return &nsv1.ShareArtifactResponse{Permission: perm}, nil
}

func (s *Service) RevokeArtifactShare(ctx context.Context, req *nsv1.RevokeArtifactShareRequest) (*nsv1.RevokeArtifactShareResponse, error) {
	if req.GetArtifactId() == "" || req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id and user_id required")
	}
	art, err := s.loadArtifact(ctx, req.GetArtifactId())
	if err != nil {
		return nil, err
	}
	viewerID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	if !canAdmin(art, viewerID) {
		grants, lerr := s.listGrants(ctx, art.GetId())
		if lerr != nil {
			return nil, lerr
		}
		p := verifiers.FromContext(ctx)
		if !canRead(p, art, grants, viewerID) {
			return nil, notFoundForRead()
		}
		return nil, status.Error(codes.PermissionDenied, "owner only: revoke")
	}
	if err := s.records.Delete(ctx, permissionsCollection, permissionKey(art.GetId(), req.GetUserId()), nil); err != nil && !errors.Is(err, records.ErrNotFound) {
		return nil, recordErr(err)
	}
	return &nsv1.RevokeArtifactShareResponse{}, nil
}

func (s *Service) ListArtifactPermissions(ctx context.Context, req *nsv1.ListArtifactPermissionsRequest) (*nsv1.ListArtifactPermissionsResponse, error) {
	if req.GetArtifactId() == "" {
		return nil, status.Error(codes.InvalidArgument, "artifact_id required")
	}
	art, err := s.loadArtifactForRead(ctx, req.GetArtifactId())
	if err != nil {
		return nil, err
	}
	grants, err := s.listGrants(ctx, art.GetId())
	if err != nil {
		return nil, err
	}
	sort.SliceStable(grants, func(i, j int) bool { return grants[i].GetUserId() < grants[j].GetUserId() })
	return &nsv1.ListArtifactPermissionsResponse{Permissions: grants}, nil
}

// -----------------------------------------------------------------------------
// Internals
// -----------------------------------------------------------------------------

// callerUserID returns the user_id the caller acts as. For OIDC + static
// callers this is the principal id directly. For SchemeWorker callers
// the bound run's owner is resolved via RunLookup; this guarantees a
// worker credential cannot manipulate artifacts owned by a different
// user even if it forges req.owner_id.
func (s *Service) callerUserID(ctx context.Context) (string, error) {
	p := verifiers.FromContext(ctx)
	if p == nil {
		return "", status.Error(codes.Unauthenticated, "missing principal")
	}
	if p.Scheme == verifiers.SchemeWorker {
		if s.runs == nil {
			return "", status.Error(codes.FailedPrecondition, "worker callers require RunLookup")
		}
		owner, _, err := s.runs.LookupRunOwner(ctx, p.RunID)
		if err != nil {
			if errors.Is(err, records.ErrNotFound) {
				return "", status.Error(codes.NotFound, "run not found")
			}
			return "", status.Errorf(codes.Internal, "lookup run owner: %v", err)
		}
		return owner, nil
	}
	return p.ID, nil
}

// deriveOwnerRunSession returns the effective (owner_id, run_id,
// session_id) for a CreateObjectArtifact call. SchemeWorker callers
// inherit all three from the bound Run (the request fields are
// ignored — a worker cannot stamp a foreign session_id on its
// output); SchemeUser/Service callers take req.owner_id when set,
// else principal.ID, and accept req.run_id / req.session_id as
// free-form attribution.
func (s *Service) deriveOwnerRunSession(ctx context.Context, reqOwner, reqRun, reqSession string) (string, string, string, error) {
	p := verifiers.FromContext(ctx)
	if p == nil {
		return "", "", "", status.Error(codes.Unauthenticated, "missing principal")
	}
	if p.Scheme == verifiers.SchemeWorker {
		if s.runs == nil {
			return "", "", "", status.Error(codes.FailedPrecondition, "worker callers require RunLookup")
		}
		owner, session, err := s.runs.LookupRunOwner(ctx, p.RunID)
		if err != nil {
			if errors.Is(err, records.ErrNotFound) {
				return "", "", "", status.Error(codes.NotFound, "run not found")
			}
			return "", "", "", status.Errorf(codes.Internal, "lookup run owner: %v", err)
		}
		return owner, p.RunID, session, nil
	}
	owner := reqOwner
	if owner == "" {
		owner = p.ID
	}
	if owner != p.ID {
		return "", "", "", status.Error(codes.PermissionDenied, "cannot create artifacts on behalf of another user")
	}
	return owner, reqRun, reqSession, nil
}

func (s *Service) loadArtifact(ctx context.Context, id string) (*nsv1.Artifact, error) {
	rec, err := s.records.Get(ctx, artifactsCollection, id)
	if err != nil {
		if errors.Is(err, records.ErrNotFound) {
			return nil, notFoundForRead()
		}
		return nil, recordErr(err)
	}
	return recordToArtifact(rec)
}

// loadArtifactForRead enforces canRead in addition to loading.
func (s *Service) loadArtifactForRead(ctx context.Context, id string) (*nsv1.Artifact, error) {
	art, err := s.loadArtifact(ctx, id)
	if err != nil {
		return nil, err
	}
	viewerID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	grants, err := s.listGrants(ctx, art.GetId())
	if err != nil {
		return nil, err
	}
	p := verifiers.FromContext(ctx)
	if !canRead(p, art, grants, viewerID) {
		return nil, notFoundForRead()
	}
	return art, nil
}

func (s *Service) listGrants(ctx context.Context, artifactID string) ([]*nsv1.ArtifactPermission, error) {
	page, _, err := s.records.List(ctx, records.ListQuery{
		Collection: permissionsCollection,
		AttributeFilters: map[string]string{
			attrPermArtifactID: artifactID,
		},
		PageSize: 1000,
	})
	if err != nil {
		return nil, recordErr(err)
	}
	out := make([]*nsv1.ArtifactPermission, 0, len(page))
	for _, rec := range page {
		perm, err := decodePermission(rec)
		if err != nil {
			s.logger.Warn("artifacts: skipping malformed permission record", "key", rec.Key, "err", err)
			continue
		}
		out = append(out, perm)
	}
	return out, nil
}

func (s *Service) findByIdemKey(ctx context.Context, ownerID, idemKey string) (*nsv1.Artifact, string, bool, error) {
	page, _, err := s.records.List(ctx, records.ListQuery{
		Collection: artifactsCollection,
		AttributeFilters: map[string]string{
			attrOwnerID: ownerID,
			attrIdemKey: idemKey,
		},
		PageSize: 2,
	})
	if err != nil {
		return nil, "", false, recordErr(err)
	}
	if len(page) == 0 {
		return nil, "", false, nil
	}
	art, err := recordToArtifact(page[0])
	if err != nil {
		return nil, "", false, status.Errorf(codes.Internal, "decode artifact: %v", err)
	}
	return art, idemHashFromAttr(page[0].Attributes), true, nil
}

// -----------------------------------------------------------------------------
// Marshal helpers
// -----------------------------------------------------------------------------

func artifactToRecord(art *nsv1.Artifact, idemKey, contentHashHex string) (records.Record, error) {
	if art.GetId() == "" {
		return records.Record{}, errors.New("artifacts: Artifact.id required")
	}
	data, err := proto.Marshal(art)
	if err != nil {
		return records.Record{}, fmt.Errorf("marshal artifact: %w", err)
	}
	attrs := map[string]string{
		attrType:    artifactTypeAttr(art.GetType()),
		attrOwnerID: art.GetOwnerId(),
		attrPublic:  boolAttr(art.GetPublic()),
	}
	if r := art.GetRunId(); r != "" {
		attrs[attrRunID] = r
	}
	if sid := art.GetSessionId(); sid != "" {
		attrs[attrSessionID] = sid
	}
	if idemKey != "" {
		attrs[attrIdemKey] = idemKey
	}
	if contentHashHex != "" {
		attrs["content_hash"] = contentHashHex
	}
	return records.Record{
		Collection:  artifactsCollection,
		Key:         art.GetId(),
		Attributes:  attrs,
		Data:        data,
		ContentType: recordContentType,
	}, nil
}

func recordToArtifact(rec records.Record) (*nsv1.Artifact, error) {
	art := &nsv1.Artifact{}
	if err := proto.Unmarshal(rec.Data, art); err != nil {
		return nil, fmt.Errorf("unmarshal artifact: %w", err)
	}
	return art, nil
}

func permissionToRecord(perm *nsv1.ArtifactPermission) (records.Record, error) {
	if perm.GetArtifactId() == "" || perm.GetUserId() == "" {
		return records.Record{}, errors.New("artifacts: permission requires artifact_id + user_id")
	}
	data, err := proto.Marshal(perm)
	if err != nil {
		return records.Record{}, fmt.Errorf("marshal permission: %w", err)
	}
	return records.Record{
		Collection: permissionsCollection,
		Key:        permissionKey(perm.GetArtifactId(), perm.GetUserId()),
		Attributes: map[string]string{
			attrPermArtifactID: perm.GetArtifactId(),
			attrPermUserID:     perm.GetUserId(),
		},
		Data:        data,
		ContentType: recordContentType,
	}, nil
}

func decodePermission(rec records.Record) (*nsv1.ArtifactPermission, error) {
	perm := &nsv1.ArtifactPermission{}
	if err := proto.Unmarshal(rec.Data, perm); err != nil {
		return nil, fmt.Errorf("unmarshal permission: %w", err)
	}
	return perm, nil
}

// -----------------------------------------------------------------------------
// Misc helpers
// -----------------------------------------------------------------------------

// notFoundForRead collapses missing-or-no-access into NOT_FOUND
// (artifacts.md §3) so the outer surface does not leak existence.
func notFoundForRead() error {
	return status.Error(codes.NotFound, "artifact not found")
}

func artifactTypeAttr(t nsv1.ArtifactType) string {
	switch t {
	case nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT:
		return "object"
	case nsv1.ArtifactType_ARTIFACT_TYPE_APP:
		return "app"
	}
	return ""
}

func boolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func contentHash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// idemHashFromAttr is a stable accessor for the optional content_hash
// attribute that pairs with attrIdemKey. Kept as a function so the
// schema rename touches one place.
func idemHashFromAttr(attrs map[string]string) string {
	return attrs["content_hash"]
}

func recordErr(err error) error {
	switch {
	case errors.Is(err, records.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, records.ErrVersionConflict):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, records.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func objectErr(err error) error {
	switch {
	case errors.Is(err, objects.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, objects.ErrInvalidState):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, objects.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
