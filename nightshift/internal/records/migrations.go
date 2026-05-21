package records

// schemaSQL is applied on Open. Each statement is idempotent
// (IF NOT EXISTS) so startup is safe on existing databases.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS records (
  collection   TEXT    NOT NULL,
  key          TEXT    NOT NULL,
  attributes   TEXT    NOT NULL DEFAULT '{}',
  data         BLOB    NOT NULL DEFAULT X'',
  content_type TEXT    NOT NULL DEFAULT '',
  version      INTEGER NOT NULL,
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
  version     INTEGER NOT NULL,
  data_hash   TEXT    NOT NULL,
  created_at  TEXT    NOT NULL,
  PRIMARY KEY (collection, key, idem_key)
);
`
