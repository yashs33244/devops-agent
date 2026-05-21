// Package server wires gRPC + grpc-gateway + lifecycle for the
// nightshift-api binary.
package server

import (
	"log/slog"

	"google.golang.org/grpc"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// Registry bundles the service implementations the gRPC server
// registers. Fields are typed as the generated gRPC server
// interfaces so callers can supply real impls, stubs, or future
// alternate implementations interchangeably.
type Registry struct {
	Storage    nsv1.StorageServer
	Workers    nsv1.WorkersServer
	Config     nsv1.ConfigServer
	SecretsAPI nsv1.SecretsServer
	Auth       nsv1.AuthServer
	Artifacts  nsv1.ArtifactsServer
	Scheduling nsv1.SchedulingServer
}

// NewGRPC constructs a *grpc.Server wired with recovery + auth
// interceptors and all seven Nightshift services.
//
// Interceptor order: recovery is outermost so a panic anywhere
// below (including inside the auth interceptor) is caught and
// converted to codes.Internal.
func NewGRPC(logger *slog.Logger, reg *Registry, v verifiers.Set) *grpc.Server {
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			UnaryRecoveryInterceptor(logger),
			verifiers.UnaryInterceptor(v),
		),
		grpc.ChainStreamInterceptor(
			StreamRecoveryInterceptor(logger),
			verifiers.StreamInterceptor(v),
		),
	)
	nsv1.RegisterStorageServer(s, reg.Storage)
	nsv1.RegisterWorkersServer(s, reg.Workers)
	nsv1.RegisterConfigServer(s, reg.Config)
	nsv1.RegisterSecretsServer(s, reg.SecretsAPI)
	nsv1.RegisterAuthServer(s, reg.Auth)
	nsv1.RegisterArtifactsServer(s, reg.Artifacts)
	nsv1.RegisterSchedulingServer(s, reg.Scheduling)
	return s
}
