package config

import (
	"github.com/nightshiftco/nightshift/internal/oauth"
)

// signState / verifyState delegate to internal/oauth. The state scheme
// is shared with the Auth service's OAuth flow; keeping the canonical
// implementation in one place avoids drift.
func (s *Service) signState(userID, connectorName string) (string, error) {
	return oauth.SignState(s.stateKey, userID, connectorName, s.now())
}

func (s *Service) verifyState(token, expectedUser, expectedConnector string) error {
	return oauth.VerifyState(s.stateKey, token, expectedUser, expectedConnector, s.now())
}
