package broadcaster

import "testing"

// TestInMemBroadcasterCompliance exercises the full Broadcaster
// contract against the in-memory implementation. CrossInstancePublish
// is profile-gated off because two NewInMem instances are by design
// independent (no shared backend).
func TestInMemBroadcasterCompliance(t *testing.T) {
	runBroadcasterComplianceSuite(t, func(t *testing.T) Broadcaster {
		b := NewInMem()
		t.Cleanup(func() { _ = b.Close() })
		return b
	}, Profile{})
}
