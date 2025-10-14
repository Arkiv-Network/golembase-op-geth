-- name: InsertEntity :exec
INSERT INTO entities (key, expires_at, payload, owner_address, created_at_block, last_modified_at_block, transaction_index_in_block, operation_index_in_transaction) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertStringAnnotation :exec
INSERT INTO string_annotations (entity_key, annotation_key, value) VALUES (?, ?, ?);

-- name: InsertNumericAnnotation :exec
INSERT INTO numeric_annotations (entity_key, annotation_key, value) VALUES (?, ?, ?);

-- name: GetEntity :one
SELECT expires_at, payload, owner_address, created_at_block, last_modified_at_block FROM entities WHERE key = ?;

-- name: GetEntityPayload :one
SELECT payload FROM entities WHERE key = ?;

-- name: GetEntitiesByOwner :many
SELECT key, expires_at, payload, created_at_block, last_modified_at_block FROM entities WHERE owner_address = ?;

-- name: GetEntityKeysByOwner :many
SELECT key FROM entities WHERE owner_address = ? ORDER BY key;

-- name: GetStringAnnotations :many
SELECT annotation_key, value FROM string_annotations WHERE entity_key = ?;

-- name: GetNumericAnnotations :many
SELECT annotation_key, value FROM numeric_annotations WHERE entity_key = ?;

-- name: DeleteEntity :exec
DELETE FROM entities WHERE key = ?;

-- name: DeleteStringAnnotations :exec
DELETE FROM string_annotations WHERE entity_key = ?;

-- name: DeleteNumericAnnotations :exec
DELETE FROM numeric_annotations WHERE entity_key = ?;

-- name: UpdateEntityExpiresAt :exec
UPDATE entities
SET
  expires_at = ?,
  last_modified_at_block = ?,
  transaction_index_in_block = ?,
  operation_index_in_transaction = ?
WHERE key = ?;

-- name: GetProcessingStatus :one
SELECT last_processed_block_number, last_processed_block_hash FROM processing_status WHERE network = ?;

-- name: UpdateProcessingStatus :exec
UPDATE processing_status SET last_processed_block_number = ?, last_processed_block_hash = ? WHERE network = ?;

-- name: InsertProcessingStatus :exec
INSERT INTO processing_status (network, last_processed_block_number, last_processed_block_hash) VALUES (?, ?, ?);

-- name: HasProcessingStatus :one
SELECT COUNT(*) > 0 FROM processing_status WHERE network = ?;

-- name: CountNetworks :one
SELECT COUNT(DISTINCT network) FROM processing_status;

-- name: DeleteProcessingStatus :exec
DELETE FROM processing_status WHERE network = ?;

-- name: EntityExists :one
SELECT COUNT(*) > 0 FROM entities WHERE key = ?;

-- name: StringAnnotationsForEntityExists :one
SELECT COUNT(*) > 0 FROM string_annotations WHERE entity_key = ?;

-- name: NumericAnnotationsForEntityExists :one
SELECT COUNT(*) > 0 FROM numeric_annotations WHERE entity_key = ?;

-- name: DeleteAllEntities :exec
DELETE FROM entities;

-- name: DeleteAllStringAnnotations :exec
DELETE FROM string_annotations;

-- name: DeleteAllNumericAnnotations :exec
DELETE FROM numeric_annotations;

-- name: DeleteAllProcessingStatus :exec
DELETE FROM processing_status;

-- name: GetEntityMetadata :one
SELECT
  expires_at,
  owner_address,
    payload,
  created_at_block,
  last_modified_at_block
FROM entities
WHERE key = ?;

-- name: GetEntityStringAnnotations :many
SELECT
  annotation_key,
  value
FROM string_annotations
WHERE entity_key = ?
ORDER BY annotation_key;

-- name: GetEntityNumericAnnotations :many
SELECT
  annotation_key,
  value
FROM numeric_annotations
WHERE entity_key = ?
ORDER BY annotation_key;

-- name: GetEntitiesToExpireAtBlock :many
SELECT key
FROM entities
WHERE expires_at = ?
ORDER BY key;

-- name: GetEntitiesForStringAnnotation :many
SELECT entity_key
FROM string_annotations
WHERE annotation_key = ? AND value = ?
ORDER BY entity_key;

-- name: GetEntitiesForNumericAnnotation :many
SELECT entity_key
FROM numeric_annotations
WHERE annotation_key = ? AND value = ?
ORDER BY entity_key;

-- name: GetAllEntityKeys :many
SELECT key FROM entities ORDER BY key;

-- name: GetEntityCount :one
SELECT COUNT(*) FROM entities;
