package records

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite is the reference RecordStore backend. Pure-Go (no CGO).
// Uses WAL + busy_timeout for concurrent reads alongside the API's
// single writer goroutine pool.
type SQLite struct {
	db *sql.DB
}

// OpenSQLite opens or creates the SQLite database at dsn and applies
// the schema. A dsn of `:memory:` or `file::memory:?cache=shared`
// works for tests. Otherwise supply a path like `file:/var/lib/ns.db`.
func OpenSQLite(dsn string) (*SQLite, error) {
	// Apply WAL + busy_timeout pragmas via the DSN. modernc.org/sqlite
	// accepts `_pragma=journal_mode(WAL)` query parameters.
	effective := dsn
	if !strings.Contains(dsn, "_pragma=") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		effective = dsn + sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)"
	}
	db, err := sql.Open("sqlite", effective)
	if err != nil {
		return nil, fmt.Errorf("records: open sqlite: %w", err)
	}
	// Single writer; many readers. The sqlite driver serializes writes
	// but parallel reads are useful for ListRecords and GetRecord.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)

	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("records: apply schema: %w", err)
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

// Put writes the record, bumping version. ifVersion enforces
// optimistic concurrency.
func (s *SQLite) Put(ctx context.Context, r Record, ifVersion *int64, idemKey string) (Record, error) {
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

	tx, err := s.db.BeginTx(ctx, nil)
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
			WHERE collection = ? AND key = ? AND idem_key = ?`,
			r.Collection, r.Key, idemKey)
		switch err := row.Scan(&cachedVersion, &cachedHash); {
		case err == sql.ErrNoRows:
			// fall through to perform write
		case err != nil:
			return Record{}, err
		default:
			if cachedHash != hash {
				return Record{}, fmt.Errorf("idempotency replay with different payload")
			}
			// Replay: return the cached record.
			out, err := getRecordTx(ctx, tx, r.Collection, r.Key)
			if err != nil {
				return Record{}, err
			}
			if err := tx.Commit(); err != nil {
				return Record{}, err
			}
			return out, nil
		}
	}

	// Fetch existing version, if any.
	var currentVersion int64
	row := tx.QueryRowContext(ctx, `SELECT version FROM records WHERE collection = ? AND key = ?`,
		r.Collection, r.Key)
	switch err := row.Scan(&currentVersion); {
	case err == sql.ErrNoRows:
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
		// preserve created_at
		var existing string
		err := tx.QueryRowContext(ctx, `SELECT created_at FROM records WHERE collection = ? AND key = ?`,
			r.Collection, r.Key).Scan(&existing)
		if err != nil {
			return Record{}, err
		}
		t, perr := time.Parse(time.RFC3339Nano, existing)
		if perr == nil {
			createdAt = t
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO records (collection, key, attributes, data, content_type, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM record_attributes WHERE collection = ? AND key = ?`,
		r.Collection, r.Key); err != nil {
		return Record{}, err
	}
	for name, value := range r.Attributes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO record_attributes (collection, key, name, value) VALUES (?, ?, ?, ?)`,
			r.Collection, r.Key, name, value); err != nil {
			return Record{}, err
		}
	}

	// Remember the idempotency entry.
	if idemKey != "" {
		hash := dataHash(r.Data, r.ContentType, attrJSON)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO record_idempotency (collection, key, idem_key, version, data_hash, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
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

func (s *SQLite) Get(ctx context.Context, collection, key string) (Record, error) {
	return getRecord(ctx, s.db, collection, key)
}

func (s *SQLite) Delete(ctx context.Context, collection, key string, ifVersion *int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var currentVersion int64
	err = tx.QueryRowContext(ctx, `SELECT version FROM records WHERE collection = ? AND key = ?`,
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM records WHERE collection = ? AND key = ?`, collection, key); err != nil {
		return err
	}
	// FK cascades to record_attributes; idempotency entries linger by design.
	return tx.Commit()
}

func (s *SQLite) List(ctx context.Context, q ListQuery) ([]Record, string, error) {
	if q.Collection == "" {
		return nil, "", fmt.Errorf("records: collection required")
	}
	pageSize := int(q.PageSize)
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 100
	}

	// Cursor is "<updated_at_rfc3339>|<key>" base16'd (opaque to caller).
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

	var (
		conds []string
		args  []any
	)
	conds = append(conds, "r.collection = ?")
	args = append(args, q.Collection)

	for name, value := range q.AttributeFilters {
		conds = append(conds, `EXISTS (SELECT 1 FROM record_attributes ra
			WHERE ra.collection = r.collection AND ra.key = r.key AND ra.name = ? AND ra.value = ?)`)
		args = append(args, name, value)
	}

	if cursorUpdated != "" {
		cmp := "<"
		if !orderDesc {
			cmp = ">"
		}
		conds = append(conds, fmt.Sprintf("(r.%s %s ? OR (r.%s = ? AND r.key %s ?))",
			orderField, cmp, orderField, cmp))
		args = append(args, cursorUpdated, cursorUpdated, cursorKey)
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
		LIMIT ?`,
		strings.Join(conds, " AND "), orderField, dir, dir)
	args = append(args, pageSize+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
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

// RecoverStaleRuns marks any record in the `runs` collection whose
// attribute status = "RUNNING" or "PENDING" as ERROR with an error
// message. Called once at control-plane startup. Implementations that
// have a live JobLauncher may be smarter; the SQLite default is
// conservative: on startup, assume any run without fresh liveness is
// dead.
func (s *SQLite) RecoverStaleRuns(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
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
		rec, err := getRecord(ctx, s.db, "runs", key)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return count, err
		}
		// Touch the status attribute; the API layer re-serializes the
		// run payload with the new status and posts a Put.
		if rec.Attributes == nil {
			rec.Attributes = map[string]string{}
		}
		rec.Attributes["status"] = "ERROR"
		rec.Attributes["recovered"] = "true"
		if _, err := s.Put(ctx, rec, &rec.Version, ""); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func marshalAttrs(a map[string]string) (string, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalAttrs(s string) map[string]string {
	if s == "" || s == "{}" {
		return map[string]string{}
	}
	out := map[string]string{}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func cloneAttrs(a map[string]string) map[string]string {
	if a == nil {
		return nil
	}
	out := make(map[string]string, len(a))
	for k, v := range a {
		out[k] = v
	}
	return out
}

func dataHash(data []byte, contentType, attrs string) string {
	h := sha256.New()
	h.Write([]byte(contentType))
	h.Write([]byte{0})
	h.Write([]byte(attrs))
	h.Write([]byte{0})
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(sc rowScanner) (Record, error) {
	var (
		r        Record
		attrJSON string
		created  string
		updated  string
	)
	if err := sc.Scan(&r.Collection, &r.Key, &attrJSON, &r.Data, &r.ContentType, &r.Version, &created, &updated); err != nil {
		return Record{}, err
	}
	r.Attributes = unmarshalAttrs(attrJSON)
	if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
		r.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updated); err == nil {
		r.UpdatedAt = t
	}
	return r, nil
}

func getRecord(ctx context.Context, q querier, collection, key string) (Record, error) {
	row := q.QueryRowContext(ctx, `
		SELECT collection, key, attributes, data, content_type, version, created_at, updated_at
		FROM records WHERE collection = ? AND key = ?`, collection, key)
	r, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	return r, err
}

func getRecordTx(ctx context.Context, tx *sql.Tx, collection, key string) (Record, error) {
	return getRecord(ctx, tx, collection, key)
}

type querier interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

func encodeCursor(t, key string) string {
	return hex.EncodeToString([]byte(t + "|" + key))
}

func decodeCursor(s string) (string, string, error) {
	if s == "" {
		return "", "", nil
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid cursor")
	}
	return parts[0], parts[1], nil
}

func timeRFC(r Record, field string) string {
	t := r.UpdatedAt
	if field == "created_at" {
		t = r.CreatedAt
	}
	return t.UTC().Format(time.RFC3339Nano)
}
