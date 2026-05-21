//go:build integration

package records

import (
	"context"
	"testing"
)

// newPostgresStore returns the package-singleton Postgres store and
// registers a TRUNCATE in t.Cleanup so each subtest gets a fresh
// blank database. Faster + simpler than a container-per-subtest at
// the cost of running tests serially (compliance suite already
// doesn't t.Parallel).
func newPostgresStore(t *testing.T) RecordStore {
	t.Helper()
	t.Cleanup(func() {
		// TRUNCATE all three tables in one statement. CASCADE handles
		// the FK from record_attributes; RESTART IDENTITY is no-op
		// today (no SERIAL columns) but free insurance against future
		// migrations adding one.
		_, err := pgStore.db.ExecContext(context.Background(),
			`TRUNCATE records, record_attributes, record_idempotency RESTART IDENTITY CASCADE`)
		if err != nil {
			t.Logf("postgres TRUNCATE cleanup: %v", err)
		}
	})
	return pgStore
}

func TestPostgresCompliance(t *testing.T) {
	runComplianceSuite(t, newPostgresStore)
}
