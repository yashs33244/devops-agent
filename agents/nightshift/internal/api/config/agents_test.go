package config

import (
	"testing"

	"google.golang.org/grpc/codes"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

func TestAgent_CreateGetListUpdateDelete(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := ctxAs(verifiers.SchemeUser, "alice")

	cr, err := svc.CreateAgent(ctx, &nsv1.CreateAgentRequest{
		UserId:      "alice",
		Name:        "summarizer",
		Description: "summarize stuff",
		Prompt:      "you are a summarizer",
		Tools:       []string{"Read"},
		Model:       "haiku",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if cr.GetAgent().GetId() == "" {
		t.Fatalf("expected id assigned")
	}

	got, err := svc.GetAgent(ctx, &nsv1.GetAgentRequest{AgentId: cr.GetAgent().GetId()})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetAgent().GetName() != "summarizer" {
		t.Fatalf("name=%q", got.GetAgent().GetName())
	}

	lr, err := svc.ListAgents(ctx, &nsv1.ListAgentsRequest{UserId: "alice", PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lr.GetAgents()) != 1 {
		t.Fatalf("agents=%d", len(lr.GetAgents()))
	}

	newPrompt := "rewritten"
	ur, err := svc.UpdateAgent(ctx, &nsv1.UpdateAgentRequest{
		AgentId: cr.GetAgent().GetId(),
		Prompt:  &newPrompt,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if ur.GetAgent().GetPrompt() != "rewritten" {
		t.Fatalf("prompt=%q", ur.GetAgent().GetPrompt())
	}

	if _, err := svc.DeleteAgent(ctx, &nsv1.DeleteAgentRequest{AgentId: cr.GetAgent().GetId()}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetAgent(ctx, &nsv1.GetAgentRequest{AgentId: cr.GetAgent().GetId()}); err == nil {
		t.Fatalf("expected get-after-delete to fail")
	}
}

func TestAgent_OwnerCheck(t *testing.T) {
	svc, _, _ := newTestService(t)

	// Bob can't create an agent for Alice.
	bob := ctxAs(verifiers.SchemeUser, "bob")
	_, err := svc.CreateAgent(bob, &nsv1.CreateAgentRequest{
		UserId: "alice", Name: "x", Prompt: "p",
	})
	mustCode(t, err, codes.PermissionDenied)

	// Alice creates one. Bob can't read it.
	alice := ctxAs(verifiers.SchemeUser, "alice")
	cr, _ := svc.CreateAgent(alice, &nsv1.CreateAgentRequest{
		UserId: "alice", Name: "x", Prompt: "p",
	})
	_, err = svc.GetAgent(bob, &nsv1.GetAgentRequest{AgentId: cr.GetAgent().GetId()})
	mustCode(t, err, codes.PermissionDenied)

	// Admin (via group) can read Alice's agent.
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	if _, err := svc.GetAgent(admin, &nsv1.GetAgentRequest{AgentId: cr.GetAgent().GetId()}); err != nil {
		t.Fatalf("admin get: %v", err)
	}
}

func TestAgent_DuplicateName(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := ctxAs(verifiers.SchemeUser, "alice")

	_, err := svc.CreateAgent(ctx, &nsv1.CreateAgentRequest{UserId: "alice", Name: "dupe", Prompt: "p"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = svc.CreateAgent(ctx, &nsv1.CreateAgentRequest{UserId: "alice", Name: "dupe", Prompt: "p"})
	mustCode(t, err, codes.AlreadyExists)
}

func TestAgent_ValidateInput(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := ctxAs(verifiers.SchemeUser, "alice")
	cases := []struct {
		req  *nsv1.CreateAgentRequest
		want codes.Code
	}{
		{&nsv1.CreateAgentRequest{Name: "x", Prompt: "p"}, codes.InvalidArgument},                         // missing user
		{&nsv1.CreateAgentRequest{UserId: "alice", Name: "BadName!", Prompt: "p"}, codes.InvalidArgument}, // bad name chars
		{&nsv1.CreateAgentRequest{UserId: "alice", Name: "ok", Prompt: ""}, codes.InvalidArgument},        // missing prompt
	}
	for _, tc := range cases {
		_, err := svc.CreateAgent(ctx, tc.req)
		mustCode(t, err, tc.want)
	}
}
