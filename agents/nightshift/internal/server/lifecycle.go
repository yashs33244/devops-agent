package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
)

// Lifecycle runs the gRPC + HTTP gateway + optional metrics listener
// + signal handler as an errgroup-style coordinated set. Blocks until
// SIGINT/SIGTERM or any component returns an error, then drains in
// reverse start order: HTTP first (so in-flight REST/SSE completes),
// then metrics, then gRPC.
type Lifecycle struct {
	Logger       *slog.Logger
	GRPCServer   *grpc.Server
	GRPCListener net.Listener
	HTTPServer   *http.Server

	// MetricsServer is optional. When set, runs alongside the HTTP
	// gateway on a separate addr. Chunk 18 mounts /metrics on this.
	MetricsServer *http.Server

	DrainTimeout time.Duration // graceful shutdown budget
}

// Run blocks until shutdown. Returns the first non-nil error.
func (l *Lifecycle) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if l.DrainTimeout == 0 {
		l.DrainTimeout = 15 * time.Second
	}

	var (
		wg       sync.WaitGroup
		errMu    sync.Mutex
		firstErr error
	)
	record := func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		if firstErr == nil && err != nil {
			firstErr = err
		}
	}

	// gRPC
	wg.Add(1)
	go func() {
		defer wg.Done()
		l.Logger.Info("grpc listening", "addr", l.GRPCListener.Addr().String())
		if err := l.GRPCServer.Serve(l.GRPCListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			record(err)
			stop()
		}
	}()

	// HTTP
	wg.Add(1)
	go func() {
		defer wg.Done()
		l.Logger.Info("http listening", "addr", l.HTTPServer.Addr)
		if err := l.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			record(err)
			stop()
		}
	}()

	// Metrics (optional)
	if l.MetricsServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Logger.Info("metrics listening", "addr", l.MetricsServer.Addr)
			if err := l.MetricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				record(err)
				stop()
			}
		}()
	}

	<-ctx.Done()
	l.Logger.Info("shutting down")

	// Drain HTTP first (in-flight REST / SSE finish), then metrics, then gRPC.
	httpCtx, httpCancel := context.WithTimeout(context.Background(), l.DrainTimeout)
	defer httpCancel()
	if err := l.HTTPServer.Shutdown(httpCtx); err != nil {
		record(err)
	}
	if l.MetricsServer != nil {
		mCtx, mCancel := context.WithTimeout(context.Background(), l.DrainTimeout)
		defer mCancel()
		if err := l.MetricsServer.Shutdown(mCtx); err != nil {
			record(err)
		}
	}

	grpcDone := make(chan struct{})
	go func() {
		l.GRPCServer.GracefulStop()
		close(grpcDone)
	}()
	select {
	case <-grpcDone:
	case <-time.After(l.DrainTimeout):
		l.Logger.Warn("grpc graceful stop timed out; forcing")
		l.GRPCServer.Stop()
	}

	wg.Wait()
	return firstErr
}
