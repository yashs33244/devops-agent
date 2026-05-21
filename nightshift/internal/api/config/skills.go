package config

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// CreateSkill registers a new skill under req.user_id. Owner write.
func (s *Service) CreateSkill(ctx context.Context, req *nsv1.CreateSkillRequest) (*nsv1.CreateSkillResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	name, ok := trimAndValidateName(req.GetName())
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "name required (lowercase alnum + dash/underscore, ≤64)")
	}
	if req.GetContent() == "" {
		return nil, status.Error(codes.InvalidArgument, "content required")
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), req.GetUserId()); err != nil {
		return nil, err
	}

	if existing, err := s.findSkillByName(ctx, req.GetUserId(), name); err != nil {
		return nil, recordErr(err)
	} else if existing != nil {
		return nil, status.Errorf(codes.AlreadyExists, "skill %q already exists for user", name)
	}

	now := s.now()
	sk := &nsv1.Skill{
		Id:          s.newID(),
		UserId:      req.GetUserId(),
		Name:        name,
		Description: req.GetDescription(),
		Content:     req.GetContent(),
		CreatedAt:   timestamppb.New(now),
		UpdatedAt:   timestamppb.New(now),
	}
	rec, err := skillToRecord(sk)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	zero := int64(0)
	if _, err := s.Records.Put(ctx, rec, &zero, req.GetIdempotencyKey()); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.CreateSkillResponse{Skill: sk}, nil
}

// GetSkill reads a skill by id.
func (s *Service) GetSkill(ctx context.Context, req *nsv1.GetSkillRequest) (*nsv1.GetSkillResponse, error) {
	if req.GetSkillId() == "" {
		return nil, status.Error(codes.InvalidArgument, "skill_id required")
	}
	rec, err := s.Records.Get(ctx, skillsCollection, req.GetSkillId())
	if err != nil {
		return nil, recordErr(err)
	}
	sk, err := recordToSkill(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), sk.GetUserId()); err != nil {
		return nil, err
	}
	return &nsv1.GetSkillResponse{Skill: sk}, nil
}

// ListSkills returns the caller's skills.
func (s *Service) ListSkills(ctx context.Context, req *nsv1.ListSkillsRequest) (*nsv1.ListSkillsResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), req.GetUserId()); err != nil {
		return nil, err
	}
	page, next, err := s.Records.List(ctx, records.ListQuery{
		Collection:       skillsCollection,
		AttributeFilters: map[string]string{attrUserID: req.GetUserId()},
		PageSize:         req.GetPageSize(),
		PageToken:        req.GetPageToken(),
	})
	if err != nil {
		return nil, recordErr(err)
	}
	out := make([]*nsv1.Skill, 0, len(page))
	for _, r := range page {
		sk, err := recordToSkill(r)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%s", err.Error())
		}
		out = append(out, sk)
	}
	return &nsv1.ListSkillsResponse{Skills: out, NextPageToken: next}, nil
}

// UpdateSkill modifies fields. Unset fields are preserved.
func (s *Service) UpdateSkill(ctx context.Context, req *nsv1.UpdateSkillRequest) (*nsv1.UpdateSkillResponse, error) {
	if req.GetSkillId() == "" {
		return nil, status.Error(codes.InvalidArgument, "skill_id required")
	}
	rec, err := s.Records.Get(ctx, skillsCollection, req.GetSkillId())
	if err != nil {
		return nil, recordErr(err)
	}
	sk, err := recordToSkill(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), sk.GetUserId()); err != nil {
		return nil, err
	}

	if req.Name != nil {
		newName, ok := trimAndValidateName(req.GetName())
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid name")
		}
		if newName != sk.Name {
			if existing, err := s.findSkillByName(ctx, sk.GetUserId(), newName); err != nil {
				return nil, recordErr(err)
			} else if existing != nil && existing.GetId() != sk.GetId() {
				return nil, status.Errorf(codes.AlreadyExists, "skill %q already exists for user", newName)
			}
			sk.Name = newName
		}
	}
	if req.Description != nil {
		sk.Description = req.GetDescription()
	}
	if req.Content != nil {
		sk.Content = req.GetContent()
	}
	sk.UpdatedAt = timestamppb.New(s.now())

	newRec, err := skillToRecord(sk)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	v := rec.Version
	if _, err := s.Records.Put(ctx, newRec, &v, ""); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.UpdateSkillResponse{Skill: sk}, nil
}

// DeleteSkill removes a skill.
func (s *Service) DeleteSkill(ctx context.Context, req *nsv1.DeleteSkillRequest) (*nsv1.DeleteSkillResponse, error) {
	if req.GetSkillId() == "" {
		return nil, status.Error(codes.InvalidArgument, "skill_id required")
	}
	rec, err := s.Records.Get(ctx, skillsCollection, req.GetSkillId())
	if err != nil {
		return nil, recordErr(err)
	}
	sk, err := recordToSkill(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), sk.GetUserId()); err != nil {
		return nil, err
	}
	v := rec.Version
	if err := s.Records.Delete(ctx, skillsCollection, req.GetSkillId(), &v); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.DeleteSkillResponse{}, nil
}

func (s *Service) findSkillByName(ctx context.Context, userID, name string) (*nsv1.Skill, error) {
	page, _, err := s.Records.List(ctx, records.ListQuery{
		Collection: skillsCollection,
		AttributeFilters: map[string]string{
			attrUserID: userID,
			attrName:   name,
		},
		PageSize: 1,
	})
	if err != nil {
		return nil, err
	}
	if len(page) == 0 {
		return nil, nil
	}
	return recordToSkill(page[0])
}
