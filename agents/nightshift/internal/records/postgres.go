package records

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	// pgx's database/sql adapter — registered as the "pgx" driver.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Postgres is the Postgres-backed RecordStore implementation. Sibling
// to *SQLite; intended for multi-replica nightshift-api deployments
// where SQLite's single-writer constraint is unworkable.
//
// Mostly a mechanical port of sqlite.go: same query shapes, same
// transaction boundaries. Differences:
//
//   - Numbered placeholders ($1, $2, …) in place of `?`.
//   - SELECT … FOR UPDATE on the version-check row read inside Put,
//     so concurrent Puts on the same (collection, key) under
//     READ COMMITTED isolation match SQLite's effectively-serializable
//     observable semantics.
//   - Schema constant lives in migrations_postgres.go.
type Postgres struct {
	db *sql.DB
}

// OpenPostgres opens a Postgres-backed store, configures the
// connection pool, and applies the schema (idempotent on existing
// databases). The DSN follows the standard libpq URL form:
//
//	postgres://user:password@host:5432/dbname?sslmode=require
//
// or libpq key/value form. Both work via pgx's database/sql adapter.
func OpenPostgres(dsn string) (*Postgres, error) {
	if dsn == "" {
		return nil, errors.New("records: OpenPostgres requires a non-empty DSN")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("records: sql.Open pgx: %w", err)
	}
	// Pool tuning: 20 max conns is a sane control-plane value that
	// won't blow past PG's default max_connections=100 with 4
	// replicas. Lifetime caps protect against PgBouncer / proxy
	// staleness; idle-time matches typical pod patterns.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("records: ping postgres: %w", err)
	}
	if _, err := db.ExecContext(ctx, schemaPostgresSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("records: apply schema: %w", err)
	}
	return &Postgres{db: db}, nil
}

// Close releases the connection pool. Safe to call on a nil receiver.
func (p *Postgres) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

// Put writes the record, bumping version. ifVersion enforces optimistic
// concurrency. Same semantics as SQLite.Put. The only implementation
// difference is the SELECT … FOR UPDATE on the version-row read so
// concurrent Puts on the same (collection, key) under READ COMMITTED
// serialize at row level — matching SQLite's effectively-serializable
// observable behavior.
func (p *Postgres) Put(ctx context.Context, r Record, ifVersion *int64, idemKey string) (Record, error) {
	if r.Collection == "" || r.Key == "" {
		return Record{}, fmt.Errorf("%w: collection and key required", ErrNotFound)
	}
	if r.Data == nil {
		r.Data = []byte{}
	}
	attrJSON, err := marshalAttrs(r.Attributes)
	if err != nil {
		return Record{}, err
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Idempotency check (if key provided).
	if idemKey != "" {
		hash := dataHash(r.Data, r.ContentType, attrJSON)
		var cachedVersion int64
		var cachedHash string
		row := tx.QueryRowContext(ctx, `SELECT version, data_hash FROM record_idempotency
			WHERE collection = $1 AND key = $2 AND idem_key = $3`,
			r.Collection, r.Key, idemKey)
		switch err := row.Scan(&cachedVersion, &cachedHash); {
		case errors.Is(err, sql.ErrNoRows):
			// fall through to perform write
		case err != nil:
			return Record{}, err
		default:
			if cachedHash != hash {
				return Record{}, fmt.Errorf("idempotency replay with different payload")
			}
			out, err := getRecordPgTx(ctx, tx, r.Collection, r.Key)
			if err != nil {
				return Record{}, err
			}
			if err := tx.Commit(); err != nil {
				return Record{}, err
			}
			return out, nil
		}
	}

	// Lock-and-fetch existing version. FOR UPDATE serializes
	// concurrent Puts at row level under READ COMMITTED isolation.
	var currentVersion int64
	row := tx.QueryRowContext(ctx,
		`SELECT version FROM records WHERE collection = $1 AND key = $2 FOR UPDATE`,
		r.Collection, r.Key)
	switch err := row.Scan(&currentVersion); {
	case errors.Is(err, sql.ErrNoRows):
		currentVersion = 0
	case err != nil:
		return Record{}, err
	}

	if ifVersion != nil && *ifVersion != currentVersion {
		return Record{}, ErrVersionConflict
	}

	now := time.Now().UTC()
	newVersion := currentVersion + 1
	createdAt := now
	if currentVersion > 0 {
		var existing string
		err := tx.QueryRowContext(ctx,
			`SELECT created_at FROM records WHERE collection = $1 AND key = $2`,
			r.Collection, r.Key).Scan(&existing)
		if err != nil {
			return Record{}, err
		}
		if t, perr := time.Parse(time.RFC3339Nano, existing); perr == nil {
			createdAt = t
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO records (collection, key, attributes, data, content_type, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT(collection, key) DO UPDATE SET
			attributes = excluded.attributes,
			data = excluded.data,
			content_type = excluded.content_type,
			version = excluded.version,
			updated_at = excluded.updated_at`,
		r.Collection, r.Key, attrJSON, r.Data, r.ContentType,
		newVersion, createdAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return Record{}, err
	}

	// Replace the attribute index.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM record_attributes WHERE collection = $1 AND key = $2`,
		r.Collection, r.Key); err != nil {
		return Record{}, err
	}
	for name, value := range r.Attributes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO record_attributes (collection, key, name, value) VALUES ($1, $2, $3, $4)`,
			r.Collection, r.Key, name, value); err != nil {
			return Record{}, err
		}
	}

	// Idempotency cache entry.
	if idemKey != "" {
		hash := dataHash(r.Data, r.ContentType, attrJSON)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO record_idempotency (collection, key, idem_key, version, data_hash, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			r.Collection, r.Key, idemKey, newVersion, hash, now.Format(time.RFC3339Nano)); err != nil {
			return Record{}, err
		}
	}

	out := Record{
		Collection:  r.Collection,
		Key:         r.Key,
		Attributes:  cloneAttrs(r.Attributes),
		Data:        append([]byte(nil), r.Data...),
		ContentType: r.ContentType,
		Version:     newVersion,
		CreatedAt:   createdAt,
		UpdatedAt:   now,
	}
	if err := tx.Commit(); err != nil {
		return Record{}, err
	}
	return out, nil
}

func (p *Postgres) Get(ctx context.Context, collection, key string) (Record, error) {
	return getRecordPg(ctx, p.db, collection, key)
}

func (p *Postgres) Delete(ctx context.Context, collection, key string, ifVersion *int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var currentVersion int64
	err = tx.QueryRowContext(ctx,
		`SELECT version FROM records WHERE collection = $1 AND key = $2 FOR UPDATE`,
		collection, key).Scan(&currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if ifVersion != nil && *ifVersion != currentVersion {
		return ErrVersionConflict
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM records WHERE collection = $1 AND key = $2`, collection, key); err != nil {
		return err
	}
	// FK ON DELETE CASCADE removes record_attributes rows; idempotency
	// entries linger by design (replay window outlives a delete).
	return tx.Commit()
}

func (p *Postgres) List(ctx context.Context, q ListQuery) ([]Record, string, error) {
	if q.Collection == "" {
		return nil, "", fmt.Errorf("records: collection required")
	}
	pageSize := int(q.PageSize)
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 100
	}

	cursorUpdated, cursorKey, err := decodeCursor(q.PageToken)
	if err != nil {
		return nil, "", fmt.Errorf("records: bad page token: %w", err)
	}

	orderDesc := true
	orderField := "updated_at"
	if q.OrderBy != "" {
		parts := strings.Fields(q.OrderBy)
		if len(parts) >= 1 && (parts[0] == "created_at" || parts[0] == "updated_at") {
			orderField = parts[0]
		}
		if len(parts) >= 2 && strings.EqualFold(parts[1], "asc") {
			orderDesc = false
		}
	}

	// Numbered-placeholder builder: each call appends an arg and
	// returns the matching `$N` token, so the WHERE-builder doesn't
	// have to track index arithmetic by hand.
	var args []any
	ph := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	var conds []string
	conds = append(conds, "r.collection = "+ph(q.Collection))

	for name, value := range q.AttributeFilters {
		conds = append(conds, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM record_attributes ra WHERE ra.collection = r.collection AND ra.key = r.key AND ra.name = %s AND ra.value = %s)`,
			ph(name), ph(value)))
	}

	if cursorUpdated != "" {
		cmp := "<"
		if !orderDesc {
			cmp = ">"
		}
		conds = append(conds, fmt.Sprintf("(r.%s %s %s OR (r.%s = %s AND r.key %s %s))",
			orderField, cmp, ph(cursorUpdated),
			orderField, ph(cursorUpdated),
			cmp, ph(cursorKey)))
	}

	dir := "DESC"
	if !orderDesc {
		dir = "ASC"
	}
	query := fmt.Sprintf(`
		SELECT r.collection, r.key, r.attributes, r.data, r.content_type, r.version, r.created_at, r.updated_at
		FROM records r
		WHERE %s
		ORDER BY r.%s %s, r.key %s
		LIMIT %s`,
		strings.Join(conds, " AND "), orderField, dir, dir, ph(pageSize+1))

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rows.Close() }()

	out := make([]Record, 0, pageSize)
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	nextToken := ""
	if len(out) > pageSize {
		last := out[pageSize-1]
		nextToken = encodeCursor(timeRFC(last, orderField), last.Key)
		out = out[:pageSize]
	}
	return out, nextToken, nil
}

// RecoverStaleRuns mirrors SQLite.RecoverStaleRuns: select runs whose
// status is RUNNING/PENDING, then fetch-modify-Put each one through
// the standard write path so the attribute index + version-bump +
// timestamp refresh all flow through one code path.
func (p *Postgres) RecoverStaleRuns(ctx context.Context) (int, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT r.key FROM records r
		JOIN record_attributes ra ON ra.collection = r.collection AND ra.key = r.key
		WHERE r.collection = 'runs' AND ra.name = 'status' AND ra.value IN ('RUNNING', 'PENDING')`)
	if err != nil {
		return 0, err
	}
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			_ = rows.Close()
			return 0, err
		}
		keys = append(keys, k)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0
	for _, key := range keys {
		rec, err := getRecordPg(ctx, p.db, "runs", key)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return count, err
		}
		if rec.Attributes == nil {
			rec.Attributes = map[string]string{}
		}
		rec.Attributes["status"] = "ERROR"
		rec.Attributes["recovered"] = "true"
		if _, err := p.Put(ctx, rec, &rec.Version, ""); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// getRecordPg is the Postgres-flavored sibling of getRecord (sqlite.go);
// the only diff is `?` → `$1, $2`. Per the locked decision to keep
// queries native per impl.
func getRecordPg(ctx context.Context, q querier, collection, key string) (Record, error) {
	row := q.QueryRowContext(ctx, `
		SELECT collection, key, attributes, data, content_type, version, created_at, updated_at
		FROM records WHERE collection = $1 AND key = $2`, collection, key)
	r, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	return r, err
}

func getRecordPgTx(ctx context.Context, tx *sql.Tx, collection, key string) (Record, error) {
	return getRecordPg(ctx, tx, collection, key)
}
