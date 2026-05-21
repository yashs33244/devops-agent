package config

import (
	"testing"

	"google.golang.org/grpc/codes"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

func TestSkill_Lifecycle(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := ctxAs(verifiers.SchemeUser, "alice")

	cr, err := svc.CreateSkill(ctx, &nsv1.CreateSkillRequest{
		UserId: "alice", Name: "summarize", Content: "---\nname: summarize\n---\nbody",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := cr.GetSkill().GetId()

	got, err := svc.GetSkill(ctx, &nsv1.GetSkillRequest{SkillId: id})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetSkill().GetContent() == "" {
		t.Fatalf("content empty")
	}

	newContent := "edited"
	if _, err := svc.UpdateSkill(ctx, &nsv1.UpdateSkillRequest{SkillId: id, Content: &newContent}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := svc.DeleteSkill(ctx, &nsv1.DeleteSkillRequest{SkillId: id}); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestSkill_OwnerCheck(t *testing.T) {
	svc, _, _ := newTestService(t)
	bob := ctxAs(verifiers.SchemeUser, "bob")
	_, err := svc.CreateSkill(bob, &nsv1.CreateSkillRequest{UserId: "alice", Name: "x", Content: "y"})
	mustCode(t, err, codes.PermissionDenied)
}
