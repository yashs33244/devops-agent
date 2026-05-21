//go:build integration

package records

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// pgStore is the singleton Postgres-backed RecordStore shared by all
// compliance subtests in this package. Initialized in TestMain so a
// single container is reused across the suite (per-subtest isolation
// is via TRUNCATE in the per-subtest factory's t.Cleanup).
var pgStore *Postgres

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("nightshift_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("postgres testcontainer: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("postgres dsn: %v", err)
	}

	pgStore, err = OpenPostgres(dsn)
	if err != nil {
		// Tear down before failing so we don't leak a container.
		_ = container.Terminate(ctx)
		log.Fatalf("OpenPostgres: %v", err)
	}

	code := m.Run()

	if err := pgStore.Close(); err != nil {
		log.Printf("postgres close: %v", err)
	}
	if err := container.Terminate(ctx); err != nil {
		log.Printf("postgres terminate: %v", err)
	}
	os.Exit(code)
}
