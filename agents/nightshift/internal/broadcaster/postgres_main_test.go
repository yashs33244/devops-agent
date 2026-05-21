//go:build integration

package broadcaster

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// pgDSN is the connection string for the package-shared Postgres
// testcontainer. Drivers (postgres_test.go) call NewPostgres with
// this DSN per subtest. LISTEN/NOTIFY uses the global
// `ns_run_events` channel; per-subtest isolation comes from random
// runIDs (uuid) and t.Cleanup-driven Close on each broadcaster.
var pgDSN string

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("broadcaster_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("postgres testcontainer: %v", err)
	}

	pgDSN, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		log.Fatalf("postgres dsn: %v", err)
	}

	code := m.Run()

	if err := container.Terminate(ctx); err != nil {
		log.Printf("postgres terminate: %v", err)
	}
	os.Exit(code)
}
