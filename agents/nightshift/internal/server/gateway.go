package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// NewGateway builds the grpc-gateway HTTP mux that transcodes REST
// to the in-process gRPC server. `grpcAddr` is the local gRPC address
// the gateway dials (typically 127.0.0.1:50051).
func NewGateway(ctx context.Context, grpcAddr string) (*runtime.ServeMux, error) {
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := nsv1.RegisterStorageHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return nil, fmt.Errorf("register storage gateway: %w", err)
	}
	if err := nsv1.RegisterWorkersHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return nil, fmt.Errorf("register workers gateway: %w", err)
	}
	if err := nsv1.RegisterConfigHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return nil, fmt.Errorf("register config gateway: %w", err)
	}
	if err := nsv1.RegisterSecretsHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return nil, fmt.Errorf("register secrets gateway: %w", err)
	}
	if err := nsv1.RegisterAuthHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return nil, fmt.Errorf("register auth gateway: %w", err)
	}
	if err := nsv1.RegisterArtifactsHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return nil, fmt.Errorf("register artifacts gateway: %w", err)
	}
	if err := nsv1.RegisterSchedulingHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return nil, fmt.Errorf("register scheduling gateway: %w", err)
	}
	return mux, nil
}

// HealthzHandler returns 200 OK. Kubernetes liveness/readiness.
func HealthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}
