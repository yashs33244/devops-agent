package config

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/records"
)

// Storage collection names (config.md §6).
const (
	agentsCollection     = "agents"
	skillsCollection     = "skills"
	connectorsCollection = "connectors"
)

// Attribute keys.
const (
	attrUserID   = "user_id"
	attrName     = "name"
	attrAuthType = "auth_type"
)

// recordContentType matches workers' Run/Event payloads.
const recordContentType = "application/x-protobuf"

// Per-user connector credential paths in the Secrets backend.
//
//	secret/nightshift/tokens/<user_id>/<connector_name>
//
// Matches the chunk-10a-provisioned `nightshift-api` ACL policy.
const tokenPathPrefix = "secret/nightshift/tokens"

func tokenPath(userID, connectorName string) string {
	return fmt.Sprintf("%s/%s/%s", tokenPathPrefix, userID, connectorName)
}

// connectorAdminPath is where operators pre-seed a connector's OAuth
// admin credential (client_id + client_secret). Catalog reconciliation
// reads this at startup and registers an OAuth server with the
// dispenser.
//
//	secret/nightshift/connectors/<connector_name>
const connectorAdminPathPrefix = "secret/nightshift/connectors"

func connectorAdminPath(connectorName string) string {
	return fmt.Sprintf("%s/%s", connectorAdminPathPrefix, connectorName)
}

// oauthCredName is the flat-namespace key Native uses for a per-user
// OAuth credential. cr0n parity: "<user_id>-<connector_name>".
func oauthCredName(userID, connectorName string) string {
	return fmt.Sprintf("%s-%s", userID, connectorName)
}

// -----------------------------------------------------------------------------
// Agent <-> Record
// -----------------------------------------------------------------------------

func agentToRecord(a *nsv1.Agent) (records.Record, error) {
	if a.GetId() == "" {
		return records.Record{}, errors.New("config: Agent.id required")
	}
	data, err := proto.Marshal(a)
	if err != nil {
		return records.Record{}, fmt.Errorf("marshal agent: %w", err)
	}
	return records.Record{
		Collection:  agentsCollection,
		Key:         a.GetId(),
		Data:        data,
		ContentType: recordContentType,
		Attributes: map[string]string{
			attrUserID: a.GetUserId(),
			attrName:   a.GetName(),
		},
	}, nil
}

func recordToAgent(r records.Record) (*nsv1.Agent, error) {
	a := &nsv1.Agent{}
	if err := proto.Unmarshal(r.Data, a); err != nil {
		return nil, fmt.Errorf("unmarshal agent: %w", err)
	}
	return a, nil
}

// -----------------------------------------------------------------------------
// Skill <-> Record
// -----------------------------------------------------------------------------

func skillToRecord(s *nsv1.Skill) (records.Record, error) {
	if s.GetId() == "" {
		return records.Record{}, errors.New("config: Skill.id required")
	}
	data, err := proto.Marshal(s)
	if err != nil {
		return records.Record{}, fmt.Errorf("marshal skill: %w", err)
	}
	return records.Record{
		Collection:  skillsCollection,
		Key:         s.GetId(),
		Data:        data,
		ContentType: recordContentType,
		Attributes: map[string]string{
			attrUserID: s.GetUserId(),
			attrName:   s.GetName(),
		},
	}, nil
}

func recordToSkill(r records.Record) (*nsv1.Skill, error) {
	s := &nsv1.Skill{}
	if err := proto.Unmarshal(r.Data, s); err != nil {
		return nil, fmt.Errorf("unmarshal skill: %w", err)
	}
	return s, nil
}

// -----------------------------------------------------------------------------
// Connector <-> Record
// -----------------------------------------------------------------------------

func connectorToRecord(c *nsv1.Connector) (records.Record, error) {
	if c.GetId() == "" {
		return records.Record{}, errors.New("config: Connector.id required")
	}
	data, err := proto.Marshal(c)
	if err != nil {
		return records.Record{}, fmt.Errorf("marshal connector: %w", err)
	}
	return records.Record{
		Collection:  connectorsCollection,
		Key:         c.GetId(),
		Data:        data,
		ContentType: recordContentType,
		Attributes: map[string]string{
			attrName:     c.GetName(),
			attrAuthType: c.GetAuthType().String(),
		},
	}, nil
}

func recordToConnector(r records.Record) (*nsv1.Connector, error) {
	c := &nsv1.Connector{}
	if err := proto.Unmarshal(r.Data, c); err != nil {
		return nil, fmt.Errorf("unmarshal connector: %w", err)
	}
	return c, nil
}
