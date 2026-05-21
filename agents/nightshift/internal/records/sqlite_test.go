package records

import (
	"strings"
	"testing"
)

func newSQLiteStore(t *testing.T) RecordStore {
	t.Helper()
	// Replace slashes from subtest names so the URI has no path
	// separators; cache=shared keys off this name and mode=memory means
	// no real file is touched.
	name := strings.ReplaceAll(t.Name(), "/", "_")
	s, err := OpenSQLite("file:" + name + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLiteCompliance(t *testing.T) {
	runComplianceSuite(t, newSQLiteStore)
}
