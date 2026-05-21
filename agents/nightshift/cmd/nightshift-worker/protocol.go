package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// client wraps the generated Workers gRPC client with the three
// helpers this reference worker needs: emit, cancellation poll, and
// terminal signals. Authentication is a Bearer token injected into
// every outgoing RPC via a UnaryInterceptor.
type client struct {
	conn    *grpc.ClientConn
	workers nsv1.WorkersClient
	runID   string
}

// dial opens an insecure gRPC connection to apiURL (inside cluster
// this is plaintext; TLS is an ingress concern handled out-of-band).
// Every unary RPC is wrapped with an "Authorization: Bearer <cred>"
// metadata header.
func dial(apiURL, runID, credential string) (*client, error) {
	authInterceptor := func(
		ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+credential)
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	conn, err := grpc.NewClient(apiURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(authInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", apiURL, err)
	}
	return &client{
		conn:    conn,
		workers: nsv1.NewWorkersClient(conn),
		runID:   runID,
	}, nil
}

func (c *client) Close() error { return c.conn.Close() }

// emit posts a StreamEvent. The server assigns `index`; the worker
// only sets `type`, `timestamp`, and `raw`.
func (c *client) emit(ctx context.Context, typ string, raw map[string]any) error {
	rawStruct, err := structpb.NewStruct(raw)
	if err != nil {
		return fmt.Errorf("build raw struct: %w", err)
	}
	_, err = c.workers.PostWorkerEvent(ctx, &nsv1.PostWorkerEventRequest{
		RunId: c.runID,
		Event: &nsv1.StreamEvent{
			Type:      typ,
			Timestamp: timestamppb.Now(),
			Raw:       rawStruct,
		},
	})
	return err
}

// pollCancellation checks whether the server has signaled cancellation.
func (c *client) pollCancellation(ctx context.Context) (bool, error) {
	resp, err := c.workers.GetRunCancellation(ctx, &nsv1.GetRunCancellationRequest{RunId: c.runID})
	if err != nil {
		return false, err
	}
	return resp.GetCancelled(), nil
}

// complete signals successful terminal state.
func (c *client) complete(ctx context.Context, sessionID string, usage *nsv1.RunUsage) error {
	_, err := c.workers.CompleteRun(ctx, &nsv1.CompleteRunRequest{
		RunId:     c.runID,
		SessionId: sessionID,
		Usage:     usage,
	})
	return err
}

// fail signals failure terminal state.
func (c *client) fail(ctx context.Context, errMsg string) error {
	_, err := c.workers.FailRun(ctx, &nsv1.FailRunRequest{
		RunId: c.runID,
		Error: errMsg,
	})
	return err
}
