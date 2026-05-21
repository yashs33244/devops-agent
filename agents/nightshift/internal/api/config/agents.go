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

// CreateAgent registers a new agent under req.user_id. Owner write.
func (s *Service) CreateAgent(ctx context.Context, req *nsv1.CreateAgentRequest) (*nsv1.CreateAgentResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	name, ok := trimAndValidateName(req.GetName())
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "name required (lowercase alnum + dash/underscore, ≤64)")
	}
	if req.GetPrompt() == "" {
		return nil, status.Error(codes.InvalidArgument, "prompt required")
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), req.GetUserId()); err != nil {
		return nil, err
	}

	if existing, err := s.findAgentByName(ctx, req.GetUserId(), name); err != nil {
		return nil, recordErr(err)
	} else if existing != nil {
		return nil, status.Errorf(codes.AlreadyExists, "agent %q already exists for user", name)
	}

	now := s.now()
	a := &nsv1.Agent{
		Id:          s.newID(),
		UserId:      req.GetUserId(),
		Name:        name,
		Description: req.GetDescription(),
		Prompt:      req.GetPrompt(),
		Tools:       req.GetTools(),
		Model:       req.GetModel(),
		CreatedAt:   timestamppb.New(now),
		UpdatedAt:   timestamppb.New(now),
	}
	rec, err := agentToRecord(a)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	zero := int64(0)
	if _, err := s.Records.Put(ctx, rec, &zero, req.GetIdempotencyKey()); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.CreateAgentResponse{Agent: a}, nil
}

// GetAgent reads an agent by id.
func (s *Service) GetAgent(ctx context.Context, req *nsv1.GetAgentRequest) (*nsv1.GetAgentResponse, error) {
	if req.GetAgentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id required")
	}
	rec, err := s.Records.Get(ctx, agentsCollection, req.GetAgentId())
	if err != nil {
		return nil, recordErr(err)
	}
	a, err := recordToAgent(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), a.GetUserId()); err != nil {
		return nil, err
	}
	return &nsv1.GetAgentResponse{Agent: a}, nil
}

// ListAgents returns the caller's agents (or req.user_id's, if admin).
func (s *Service) ListAgents(ctx context.Context, req *nsv1.ListAgentsRequest) (*nsv1.ListAgentsResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), req.GetUserId()); err != nil {
		return nil, err
	}
	page, next, err := s.Records.List(ctx, records.ListQuery{
		Collection:       agentsCollection,
		AttributeFilters: map[string]string{attrUserID: req.GetUserId()},
		PageSize:         req.GetPageSize(),
		PageToken:        req.GetPageToken(),
	})
	if err != nil {
		return nil, recordErr(err)
	}
	out := make([]*nsv1.Agent, 0, len(page))
	for _, r := range page {
		a, err := recordToAgent(r)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%s", err.Error())
		}
		out = append(out, a)
	}
	return &nsv1.ListAgentsResponse{Agents: out, NextPageToken: next}, nil
}

// UpdateAgent modifies fields. Unset optional fields are preserved.
func (s *Service) UpdateAgent(ctx context.Context, req *nsv1.UpdateAgentRequest) (*nsv1.UpdateAgentResponse, error) {
	if req.GetAgentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id required")
	}
	rec, err := s.Records.Get(ctx, agentsCollection, req.GetAgentId())
	if err != nil {
		return nil, recordErr(err)
	}
	a, err := recordToAgent(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), a.GetUserId()); err != nil {
		return nil, err
	}

	if req.Name != nil {
		newName, ok := trimAndValidateName(req.GetName())
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid name")
		}
		if newName != a.Name {
			if existing, err := s.findAgentByName(ctx, a.GetUserId(), newName); err != nil {
				return nil, recordErr(err)
			} else if existing != nil && existing.GetId() != a.GetId() {
				return nil, status.Errorf(codes.AlreadyExists, "agent %q already exists for user", newName)
			}
			a.Name = newName
		}
	}
	if req.Description != nil {
		a.Description = req.GetDescription()
	}
	if req.Prompt != nil {
		a.Prompt = req.GetPrompt()
	}
	if req.GetSetTools() {
		a.Tools = req.GetTools()
	}
	if req.Model != nil {
		a.Model = req.GetModel()
	}
	a.UpdatedAt = timestamppb.New(s.now())

	newRec, err := agentToRecord(a)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	v := rec.Version
	if _, err := s.Records.Put(ctx, newRec, &v, ""); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.UpdateAgentResponse{Agent: a}, nil
}

// DeleteAgent removes an agent.
func (s *Service) DeleteAgent(ctx context.Context, req *nsv1.DeleteAgentRequest) (*nsv1.DeleteAgentResponse, error) {
	if req.GetAgentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id required")
	}
	rec, err := s.Records.Get(ctx, agentsCollection, req.GetAgentId())
	if err != nil {
		return nil, recordErr(err)
	}
	a, err := recordToAgent(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), a.GetUserId()); err != nil {
		return nil, err
	}
	v := rec.Version
	if err := s.Records.Delete(ctx, agentsCollection, req.GetAgentId(), &v); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.DeleteAgentResponse{}, nil
}

// findAgentByName scans the user's agents for one with the matching
// name. Returns nil, nil when not found.
func (s *Service) findAgentByName(ctx context.Context, userID, name string) (*nsv1.Agent, error) {
	page, _, err := s.Records.List(ctx, records.ListQuery{
		Collection: agentsCollection,
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
	return recordToAgent(page[0])
}
