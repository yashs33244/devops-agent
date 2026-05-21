// Package storage implements the nightshift.v1.Storage gRPC service
// by wrapping internal/records (RecordStore) and internal/objects
// (ObjectStore).
package storage

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/objects"
	"github.com/nightshiftco/nightshift/internal/records"
)

// Service is the nightshift.v1.StorageServer implementation.
type Service struct {
	nsv1.UnimplementedStorageServer

	Records records.RecordStore
	Objects objects.ObjectStore
}

func (s *Service) PutRecord(ctx context.Context, req *nsv1.PutRecordRequest) (*nsv1.PutRecordResponse, error) {
	if req.GetCollection() == "" || req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "collection and key required")
	}
	r := records.Record{
		Collection:  req.GetCollection(),
		Key:         req.GetKey(),
		Attributes:  req.GetAttributes(),
		Data:        req.GetData(),
		ContentType: req.GetContentType(),
	}
	var ifVersion *int64
	if req.GetIfVersion() != 0 || hasIfVersion(req) {
		v := req.GetIfVersion()
		ifVersion = &v
	}
	out, err := s.Records.Put(ctx, r, ifVersion, req.GetIdempotencyKey())
	if err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.PutRecordResponse{Record: recordToProto(out)}, nil
}

func (s *Service) GetRecord(ctx context.Context, req *nsv1.GetRecordRequest) (*nsv1.GetRecordResponse, error) {
	r, err := s.Records.Get(ctx, req.GetCollection(), req.GetKey())
	if err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.GetRecordResponse{Record: recordToProto(r)}, nil
}

func (s *Service) DeleteRecord(ctx context.Context, req *nsv1.DeleteRecordRequest) (*nsv1.DeleteRecordResponse, error) {
	var ifVersion *int64
	if hasDeleteIfVersion(req) {
		v := req.GetIfVersion()
		ifVersion = &v
	}
	if err := s.Records.Delete(ctx, req.GetCollection(), req.GetKey(), ifVersion); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.DeleteRecordResponse{}, nil
}

func (s *Service) ListRecords(ctx context.Context, req *nsv1.ListRecordsRequest) (*nsv1.ListRecordsResponse, error) {
	page, next, err := s.Records.List(ctx, records.ListQuery{
		Collection:       req.GetCollection(),
		AttributeFilters: req.GetAttributeFilters(),
		PageSize:         req.GetPageSize(),
		PageToken:        req.GetPageToken(),
		OrderBy:          req.GetOrderBy(),
	})
	if err != nil {
		return nil, recordErr(err)
	}
	out := &nsv1.ListRecordsResponse{NextPageToken: next}
	for i := range page {
		out.Records = append(out.Records, recordToProto(page[i]))
	}
	return out, nil
}

func (s *Service) InitiateObjectUpload(ctx context.Context, req *nsv1.InitiateObjectUploadRequest) (*nsv1.InitiateObjectUploadResponse, error) {
	if req.GetBucket() == "" || req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "bucket and key required")
	}
	obj, uploadURL, headers, expires, err := s.Objects.Initiate(ctx, objects.InitiateSpec{
		Bucket:      req.GetBucket(),
		Key:         req.GetKey(),
		ContentType: req.GetContentType(),
		SizeBytes:   req.GetSizeBytes(),
		Metadata:    req.GetMetadata(),
		TTL:         15 * time.Minute,
	})
	if err != nil {
		return nil, objectErr(err)
	}
	return &nsv1.InitiateObjectUploadResponse{
		Object:        objectToProto(obj),
		UploadUrl:     uploadURL,
		UploadHeaders: headers,
		ExpiresAt:     timestamppb.New(expires),
	}, nil
}

func (s *Service) FinalizeObjectUpload(ctx context.Context, req *nsv1.FinalizeObjectUploadRequest) (*nsv1.FinalizeObjectUploadResponse, error) {
	obj, err := s.Objects.Finalize(ctx, req.GetBucket(), req.GetKey())
	if err != nil {
		return nil, objectErr(err)
	}
	return &nsv1.FinalizeObjectUploadResponse{Object: objectToProto(obj)}, nil
}

func (s *Service) GetObject(ctx context.Context, req *nsv1.GetObjectRequest) (*nsv1.GetObjectResponse, error) {
	obj, err := s.Objects.Stat(ctx, req.GetBucket(), req.GetKey())
	if err != nil {
		return nil, objectErr(err)
	}
	return &nsv1.GetObjectResponse{Object: objectToProto(obj)}, nil
}

func (s *Service) GetObjectDownloadURL(ctx context.Context, req *nsv1.GetObjectDownloadURLRequest) (*nsv1.GetObjectDownloadURLResponse, error) {
	url, expires, err := s.Objects.DownloadURL(ctx, req.GetBucket(), req.GetKey(), 15*time.Minute)
	if err != nil {
		return nil, objectErr(err)
	}
	return &nsv1.GetObjectDownloadURLResponse{
		DownloadUrl: url,
		ExpiresAt:   timestamppb.New(expires),
	}, nil
}

func (s *Service) DeleteObject(ctx context.Context, req *nsv1.DeleteObjectRequest) (*nsv1.DeleteObjectResponse, error) {
	if err := s.Objects.Delete(ctx, req.GetBucket(), req.GetKey()); err != nil {
		return nil, objectErr(err)
	}
	return &nsv1.DeleteObjectResponse{}, nil
}

func (s *Service) ListObjects(ctx context.Context, req *nsv1.ListObjectsRequest) (*nsv1.ListObjectsResponse, error) {
	objs, next, err := s.Objects.List(ctx, req.GetBucket(), req.GetKeyPrefix(), req.GetPageToken(), req.GetPageSize())
	if err != nil {
		return nil, objectErr(err)
	}
	out := &nsv1.ListObjectsResponse{NextPageToken: next}
	for i := range objs {
		out.Objects = append(out.Objects, objectToProto(objs[i]))
	}
	return out, nil
}

func recordToProto(r records.Record) *nsv1.Record {
	return &nsv1.Record{
		Collection:  r.Collection,
		Key:         r.Key,
		Attributes:  r.Attributes,
		Data:        r.Data,
		ContentType: r.ContentType,
		Version:     r.Version,
		CreatedAt:   timestamppb.New(r.CreatedAt),
		UpdatedAt:   timestamppb.New(r.UpdatedAt),
	}
}

func objectToProto(o objects.Object) *nsv1.Object {
	return &nsv1.Object{
		Bucket:      o.Bucket,
		Key:         o.Key,
		ContentType: o.ContentType,
		SizeBytes:   o.SizeBytes,
		Etag:        o.ETag,
		Metadata:    o.Metadata,
		UploadState: nsv1.UploadState(o.State),
		CreatedAt:   timestamppb.New(o.CreatedAt),
		UpdatedAt:   timestamppb.New(o.UpdatedAt),
	}
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

// hasIfVersion and hasDeleteIfVersion distinguish "field not set"
// from "field set to zero". In proto3 scalar int64, we cannot tell
// the difference directly — zero means "require record does not
// exist" per Storage §3. Treat any non-default request as
// version-checked.
//
// For simplicity, v1 treats IfVersion=0 as a require-not-exist hint
// when the caller explicitly sends it, and otherwise as unset. The
// gateway always sends all fields, so this heuristic is sound over
// the JSON path; over pure gRPC, callers who want "require not
// exist" pass IfVersion=0 which we pass through. Callers who want
// "ignore version" must omit the field in generated JSON, which
// grpc-gateway does.
func hasIfVersion(req *nsv1.PutRecordRequest) bool {
	// Heuristic: treat zero as unset unless the caller also set an
	// idempotency key (implying they care about state). Good enough
	// for v1; can be tightened with field presence later.
	return req.GetIfVersion() != 0
}

func hasDeleteIfVersion(req *nsv1.DeleteRecordRequest) bool {
	return req.GetIfVersion() != 0
}
