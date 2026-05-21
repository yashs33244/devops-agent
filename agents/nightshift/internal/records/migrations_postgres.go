package records

// schemaPostgresSQL mirrors schemaSQL (the SQLite schema) with the
// minimum dialect changes needed for Postgres:
//
//   - BLOB        → BYTEA
//   - INTEGER     → BIGINT
//   - BLOB X”    → BYTEA '\x'::bytea (Postgres hex-literal syntax)
//
// Timestamps stay as TEXT (RFC3339Nano strings) for parity with the
// SQLite impl. Promoting to TIMESTAMPTZ is a future migration.
//
// FK enforcement does NOT need an explicit pragma in Postgres; FKs
// are always enforced.
//
// TODO: replace this idempotent CREATE-IF-NOT-EXISTS dance with a
// proper migration tool (golang-migrate / atlas) before nightshift-api
// runs as a multi-replica deployment. Concurrent CREATE TABLE
// IF NOT EXISTS is technically race-prone on a brand-new database;
// in practice the schema lock serializes them and the operator
// restart loop covers any transient failure.
const schemaPostgresSQL = `
CREATE TABLE IF NOT EXISTS records (
  collection   TEXT    NOT NULL,
  key          TEXT    NOT NULL,
  attributes   TEXT    NOT NULL DEFAULT '{}',
  data         BYTEA   NOT NULL DEFAULT '\x'::bytea,
  content_type TEXT    NOT NULL DEFAULT '',
  version      BIGINT  NOT NULL,
  created_at   TEXT    NOT NULL,
  updated_at   TEXT    NOT NULL,
  PRIMARY KEY (collection, key)
);

CREATE INDEX IF NOT EXISTS idx_records_collection_updated
  ON records(collection, updated_at DESC);

-- attribute index table: one row per (collection, key, attr_name, attr_value)
-- enabling exact-match filtering via JOIN without a JSON-query engine.
CREATE TABLE IF NOT EXISTS record_attributes (
  collection TEXT NOT NULL,
  key        TEXT NOT NULL,
  name       TEXT NOT NULL,
  value      TEXT NOT NULL,
  PRIMARY KEY (collection, key, name),
  FOREIGN KEY (collection, key) REFERENCES records(collection, key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_record_attributes_lookup
  ON record_attributes(collection, name, value);

-- idempotency cache: remembers a (collection, key, idem_key) → response_version
-- for the implementation-defined replay window.
CREATE TABLE IF NOT EXISTS record_idempotency (
  collection  TEXT    NOT NULL,
  key         TEXT    NOT NULL,
  idem_key    TEXT    NOT NULL,
  version     BIGINT  NOT NULL,
  data_hash   TEXT    NOT NULL,
  created_at  TEXT    NOT NULL,
  PRIMARY KEY (collection, key, idem_key)
);
`
