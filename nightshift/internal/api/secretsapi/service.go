// Package secretsapi is a placeholder implementation of
// nightshift.v1.Secrets. The proto service is registered in the gRPC
// server (and grpc-gateway HTTP mux) so the surface is reachable, but
// every RPC currently returns UNIMPLEMENTED via the embedded
// UnimplementedSecretsServer — there is no in-tree consumer of
// KV-via-gRPC today (the in-process internal/secrets interface is
// what control-plane code uses; chunk-11 + chunk-12 wire OpenBao
// directly).
//
// This package exists for forward compatibility: when a future
// tooling consumer (e.g. `nightshiftctl secrets get`) lands, KV
// handlers can be implemented here against the existing
// secrets.Secrets interface.
//
// The OAuth dispenser RPCs that previously lived here have moved to
// `internal/api/authapi/` (under nightshift.v1.Auth) — operators may
// pick different backends for KV vs. OAuth dispensing.
package secretsapi

import (
	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// Service is the placeholder implementation of nightshift.v1.Secrets.
// Every RPC returns UNIMPLEMENTED via the embedded
// UnimplementedSecretsServer.
type Service struct {
	nsv1.UnimplementedSecretsServer
}
