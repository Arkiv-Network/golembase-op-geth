-- name: InsertEntity :exec
INSERT INTO entities (
  key, expires_at, payload, content_type, owner_address,
  created_at_block, last_modified_at_block, deleted,
  transaction_index_in_block, operation_index_in_transaction
)
VALUES (
  ?, ?, ?, ?, ?,
  ?, ?, ?,
  ?, ?
);

-- name: InsertStringAnnotation :exec
INSERT INTO string_annotations (
  entity_key, entity_last_modified_at_block,  annotation_key, value
) VALUES (
  ?, ?, ?, ?
);

-- name: InsertNumericAnnotation :exec
INSERT INTO numeric_annotations (
  entity_key, entity_last_modified_at_block,  annotation_key, value
) VALUES (
  ?, ?, ?, ?
);

-- name: GetEntity :one
SELECT e1.expires_at, e1.payload, e1.owner_address, e1.created_at_block, e1.last_modified_at_block
FROM entities AS e1
WHERE e1.key = sqlc.arg(key)
AND e1.deleted = FALSE
AND e1.last_modified_at_block <= sqlc.arg(block)
AND NOT EXISTS (
  SELECT 1
  FROM entities AS e2
  WHERE e2.key = e1.key
  AND e2.last_modified_at_block > e1.last_modified_at_block
  -- There is a bug in sqlc currently with repeated named args,
  -- so we resolve the named arg ourselves here.
  -- See https://github.com/sqlc-dev/sqlc/issues/4110
  AND e2.last_modified_at_block <= ?2
);

-- name: GetStringAnnotations :many
SELECT a.annotation_key, a.value
FROM string_annotations AS a INNER JOIN entities AS e
  ON a.entity_key = e.key
AND a.entity_last_modified_at_block = e.last_modified_at_block
AND e.deleted = FALSE
AND e.last_modified_at_block <= sqlc.arg(block)
AND NOT EXISTS (
  SELECT 1
  FROM entities AS e2
  WHERE e2.key = e.key
  AND e2.last_modified_at_block > e.last_modified_at_block
  AND e2.last_modified_at_block <= ?2
)
WHERE a.entity_key = ?1;

-- name: GetNumericAnnotations :many
SELECT a.annotation_key, a.value
FROM numeric_annotations AS a INNER JOIN entities AS e
  ON a.entity_key = e.key
AND a.entity_last_modified_at_block = e.last_modified_at_block
AND e.deleted = FALSE
AND e.last_modified_at_block <= sqlc.arg(block)
AND NOT EXISTS (
  SELECT 1
  FROM entities AS e2
  WHERE e2.key = e.key
  AND e2.last_modified_at_block > e.last_modified_at_block
  AND e2.last_modified_at_block <= ?2
)
WHERE a.entity_key = ?1;

-- name: DeleteEntity :exec
INSERT INTO entities (
  key, expires_at, payload, content_type, owner_address,
  created_at_block, last_modified_at_block, deleted,
  transaction_index_in_block, operation_index_in_transaction
)
SELECT
    e.key,
    e.expires_at,
    e.payload,
    e.content_type,
    e.owner_address,
    e.created_at_block,
    sqlc.arg(last_modified_at_block) AS last_modified_at_block,
    TRUE AS deleted,
    sqlc.arg(transaction_index_in_block) AS transaction_index_in_block,
    sqlc.arg(operation_index_in_transaction) AS operation_index_in_transaction
FROM entities AS e
WHERE e.key = sqlc.arg(key)
AND e.deleted = FALSE
AND NOT EXISTS (
  SELECT 1
  FROM entities AS e2
  WHERE e2.key = e.key
  AND e2.last_modified_at_block > e.last_modified_at_block
);

-- name: UpdateEntityOwner :exec
INSERT INTO entities (
  key, expires_at, payload, content_type, owner_address,
  created_at_block, last_modified_at_block, deleted,
  transaction_index_in_block, operation_index_in_transaction
)
SELECT
    e.key,
    e.expires_at,
    e.payload,
    e.content_type,
    sqlc.arg(owner_address),
    e.created_at_block,
    sqlc.arg(last_modified_at_block) AS last_modified_at_block,
    e.deleted,
    sqlc.arg(transaction_index_in_block) AS transaction_index_in_block,
    sqlc.arg(operation_index_in_transaction) AS operation_index_in_transaction
FROM entities AS e
WHERE e.key = sqlc.arg(key)
AND e.deleted = FALSE
AND NOT EXISTS (
  SELECT 1
  FROM entities AS e2
  WHERE e2.key = e.key
  AND e2.last_modified_at_block > e.last_modified_at_block
);

-- name: UpdateEntityExpiresAt :exec
INSERT INTO entities (
  key, expires_at, payload, content_type, owner_address,
  created_at_block, last_modified_at_block, deleted,
  transaction_index_in_block, operation_index_in_transaction
)
SELECT
    e.key,
    sqlc.arg(expires_at) AS expires_at,
    e.payload,
    e.content_type,
    e.owner_address,
    e.created_at_block,
    sqlc.arg(last_modified_at_block) AS last_modified_at_block,
    e.deleted,
    sqlc.arg(transaction_index_in_block) AS transaction_index_in_block,
    sqlc.arg(operation_index_in_transaction) AS operation_index_in_transaction
FROM entities AS e
WHERE e.key = sqlc.arg(key)
AND e.deleted = FALSE
AND NOT EXISTS (
  SELECT 1
  FROM entities AS e2
  WHERE e2.key = e.key
  AND e2.last_modified_at_block > e.last_modified_at_block
);

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

-- name: DeleteAllEntities :exec
DELETE FROM entities;

-- name: DeleteAllStringAnnotations :exec
DELETE FROM string_annotations;

-- name: DeleteAllNumericAnnotations :exec
DELETE FROM numeric_annotations;

-- name: DeleteAllProcessingStatus :exec
DELETE FROM processing_status;

-- name: GetAllEntityKeys :many
SELECT key
FROM entities AS e
WHERE e.deleted = FALSE
AND e.last_modified_at_block <= sqlc.arg(block)
AND NOT EXISTS (
  SELECT 1
  FROM entities AS e2
  WHERE e2.key = e.key
  AND e2.last_modified_at_block > e.last_modified_at_block
  AND e2.last_modified_at_block <= ?1
)
ORDER BY key;

-- name: GetEntityCount :one
SELECT COUNT(*)
FROM entities AS e
WHERE e.deleted = FALSE
AND e.last_modified_at_block <= sqlc.arg(block)
AND NOT EXISTS (
  SELECT 1
  FROM entities AS e2
  WHERE e2.key = e.key
  AND e2.last_modified_at_block > e.last_modified_at_block
  AND e2.last_modified_at_block <= ?1
);

-- name: DeleteEntitiesUntilBlock :exec
DELETE FROM entities AS e
WHERE e.last_modified_at_block <= sqlc.arg(block)
AND (
  EXISTS (
    SELECT 1
    FROM entities AS e2
    WHERE e2.key = e.key
    AND e2.last_modified_at_block > e.last_modified_at_block
  )
  OR e.deleted = TRUE
);

-- name: DeleteStringAnnotationsUntilBlock :exec
DELETE FROM string_annotations AS a
WHERE a.entity_last_modified_at_block <= sqlc.arg(block)
AND (
  EXISTS (
    SELECT 1
    FROM entities AS e
    WHERE e.key = a.entity_key
    AND (
      -- either there is a more recent version of the entity that this annotation
      -- belongs to
      e.last_modified_at_block > a.entity_last_modified_at_block
      -- or the entity that this annotation belongs to has been deleted
      OR (
        e.last_modified_at_block = a.entity_last_modified_at_block
        AND e.deleted = TRUE
      )
    )
  )
);

-- name: DeleteNumericAnnotationsUntilBlock :exec
DELETE FROM numeric_annotations AS a
WHERE a.entity_last_modified_at_block <= sqlc.arg(block)
AND (
  EXISTS (
    SELECT 1
    FROM entities AS e
    WHERE e.key = a.entity_key
    AND (
      -- either there is a more recent version of the entity that this annotation
      -- belongs to
      e.last_modified_at_block > a.entity_last_modified_at_block
      -- or the entity that this annotation belongs to has been deleted
      OR (
        e.last_modified_at_block = a.entity_last_modified_at_block
        AND e.deleted = TRUE
      )
    )
  )
);
