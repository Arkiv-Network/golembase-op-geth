CREATE TABLE IF NOT EXISTS processing_status (
  network TEXT NOT NULL PRIMARY KEY,
  last_processed_block_number INTEGER NOT NULL,
  last_processed_block_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS schema_versions (
  id INTEGER NOT NULL PRIMARY KEY,
  entities INTEGER
);

CREATE TABLE IF NOT EXISTS entities (
  key TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  payload BLOB NOT NULL,
  content_type TEXT NOT NULL,
  created_at_block INTEGER NOT NULL,
  last_modified_at_block INTEGER NOT NULL,
  deleted BOOLEAN NOT NULL,
  transaction_index_in_block INTEGER NOT NULL,
  operation_index_in_transaction INTEGER NOT NULL,
  owner_address TEXT NOT NULL,
  PRIMARY KEY (key, last_modified_at_block)
);

CREATE INDEX IF NOT EXISTS idx_entities_owner_address ON entities(owner_address);

CREATE TABLE IF NOT EXISTS string_annotations (
  entity_key TEXT NOT NULL,
  entity_last_modified_at_block INTEGER NOT NULL,
  annotation_key TEXT NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY (entity_key, entity_last_modified_at_block, annotation_key),
  FOREIGN KEY (entity_key, entity_last_modified_at_block) REFERENCES entities(key, last_modified_at_block)
);

CREATE TABLE IF NOT EXISTS numeric_annotations (
  entity_key TEXT NOT NULL,
  entity_last_modified_at_block INTEGER NOT NULL,
  annotation_key TEXT NOT NULL,
  value INTEGER NOT NULL,
  PRIMARY KEY (entity_key, entity_last_modified_at_block, annotation_key),
  FOREIGN KEY (entity_key, entity_last_modified_at_block) REFERENCES entities(key, last_modified_at_block)
);
