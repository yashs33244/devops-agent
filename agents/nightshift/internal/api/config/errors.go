package config

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/secrets"
)

// Sentinel errors returned by handlers, mapped to gRPC status codes by
// the recordErr helper (or returned directly when the mapping is fixed).
var (
	errUnauthenticated  = status.Error(codes.Unauthenticated, "missing principal")
	errPermissionDenied = status.Error(codes.PermissionDenied, "caller does not own this resource")
)

// recordErr maps internal/records and internal/secrets sentinels to
// gRPC codes. Mirrors workers/service.go:609.
func recordErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, records.ErrNotFound), errors.Is(err, secrets.ErrNotFound), errors.Is(err, oauth.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, records.ErrVersionConflict):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, records.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, secrets.ErrNotImplemented):
		return status.Error(codes.Unimplemented, err.Error())
	}
	return status.Errorf(codes.Internal, "%s", err.Error())
}
