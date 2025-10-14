package sqlstore

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore/sqlitegolem"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
	_ "github.com/mattn/go-sqlite3"
)

const entitiesSchemaVersion = uint64(1)

type BlockWal struct {
	BlockInfo  BlockInfo
	Operations []Operation
}
type BlockInfo struct {
	Number     uint64      `json:"number,string"`
	Hash       common.Hash `json:"hash"`
	ParentHash common.Hash `json:"parentHash"`
}

type Operation struct {
	Create *Create      `json:"create,omitempty"`
	Update *Update      `json:"update,omitempty"`
	Delete *common.Hash `json:"delete,omitempty"`
	Extend *ExtendBTL   `json:"extend,omitempty"`
}

type Create struct {
	EntityKey          common.Hash                `json:"entityKey"`
	ExpiresAtBlock     uint64                     `json:"expiresAtBlock"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
	Owner              common.Address             `json:"owner"`
	TransactionIndex   uint64                     `json:"txIndex"`
	OperationIndex     uint64                     `json:"opIndex"`
}

type Update struct {
	EntityKey          common.Hash                `json:"entityKey"`
	ExpiresAtBlock     uint64                     `json:"expiresAtBlock"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
	TransactionIndex   uint64                     `json:"txIndex"`
	OperationIndex     uint64                     `json:"opIndex"`
}

type ExtendBTL struct {
	EntityKey        common.Hash `json:"entityKey"`
	OldExpiresAt     uint64      `json:"oldExpiresAt"`
	NewExpiresAt     uint64      `json:"newExpiresAt"`
	TransactionIndex uint64      `json:"txIndex"`
	OperationIndex   uint64      `json:"opIndex"`
}

// SQLStore encapsulates the SQLite SQLStore functionality
type SQLStore struct {
	db *sql.DB
}

// NewStore creates a new ETL instance with database connection and schema setup
func NewStore(dbFile string) (*SQLStore, error) {
	dir := filepath.Dir(dbFile)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", dbFile))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Check if schema exists and apply if needed
	ctx := context.Background()

	// Check if schema is up to date
	readVersions := true
	entitiesVersion := uint64(0)

	var tableName string
	err = db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='schema_versions';
	`).Scan(&tableName)

	switch err {
	case sql.ErrNoRows:
		// In version 0, we didn't have the schema_versions table yet
		entitiesVersion = 0
		readVersions = false
		log.Warn("arkiv: no schema version info found, table missing")
	case nil:
		// The schema exists, we can read the versions from it
	default:
		// We got another error
		db.Close()
		return nil, fmt.Errorf("failed to check schema: %w", err)
	}

	if readVersions {
		err = db.QueryRowContext(
			ctx,
			`SELECT entities FROM schema_versions WHERE id = 1;`,
		).Scan(&entitiesVersion)

		switch err {
		case sql.ErrNoRows:
			entitiesVersion = 0
			log.Warn("arkiv: no schema version info found, table empty", "error", err)
		case nil:
			// We read the versions, all good
			log.Info("arkiv: schema versions read from database", "entities", entitiesVersion)
		default:
			db.Close()
			return nil, fmt.Errorf("failed to check schema: %w", err)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if entitiesVersion != entitiesSchemaVersion {
		log.Warn(
			"arkiv: entities table has an outdated schema, dropping tables",
			"existingVersion", entitiesVersion,
			"requiredVersion", entitiesSchemaVersion,
		)
		_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS entities;`)
		if err != nil {
			tx.Rollback()
			db.Close()
			return nil, fmt.Errorf("failed to drop entities table: %w", err)
		}
		_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS string_annotations;`)
		if err != nil {
			tx.Rollback()
			db.Close()
			return nil, fmt.Errorf("failed to drop string_annotations table: %w", err)
		}
		_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS numeric_annotations;`)
		if err != nil {
			tx.Rollback()
			db.Close()
			return nil, fmt.Errorf("failed to drop numeric_annotations table: %w", err)
		}
	}

	log.Info("arkiv: applying database schema")
	err = sqlitegolem.ApplySchemaTx(ctx, tx)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("failed to recreate schema: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO schema_versions (id, entities) VALUES (1, ?);`,
		entitiesSchemaVersion)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("failed to update schema versions: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("failed to recreate schema: %w", err)
	}

	log.Info("arkiv: database ready", "entitySchemaVersion", entitiesSchemaVersion)
	return &SQLStore{db: db}, nil
}

// Close closes the database connection
func (e *SQLStore) Close() error {
	return e.db.Close()
}

// GetQueries returns a new sqlitegolem.Queries instance for autocommit operations
func (e *SQLStore) GetQueries() *sqlitegolem.Queries {
	return sqlitegolem.New(e.db)
}

func (e *SQLStore) GetProcessingStatus(ctx context.Context, networkID string) (*sqlitegolem.GetProcessingStatusRow, error) {
	result, err := e.GetQueries().GetProcessingStatus(ctx, networkID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &sqlitegolem.GetProcessingStatusRow{
				LastProcessedBlockNumber: 0,
				LastProcessedBlockHash:   "",
			}, nil
		}
		return nil, err
	}
	return &result, nil
}

// GetEntitiesToExpireAtBlock retrieves all entity keys that expire at the specified block
func (e *SQLStore) GetEntitiesToExpireAtBlock(ctx context.Context, blockNumber uint64) ([]common.Hash, error) {
	keys, err := e.GetQueries().GetEntitiesToExpireAtBlock(ctx, int64(blockNumber))
	if err != nil {
		return nil, fmt.Errorf("failed to get entities expiring at block %d: %w", blockNumber, err)
	}

	// Convert string keys to common.Hash
	result := make([]common.Hash, 0, len(keys))
	for _, keyHex := range keys {
		result = append(result, common.HexToHash(keyHex))
	}

	return result, nil
}

// GetEntitiesForStringAnnotationValue retrieves all entity keys that have a specific string annotation with the given value
func (e *SQLStore) GetEntitiesForStringAnnotationValue(ctx context.Context, annotationKey, value string) ([]common.Hash, error) {
	keys, err := e.GetQueries().GetEntitiesForStringAnnotation(ctx, sqlitegolem.GetEntitiesForStringAnnotationParams{
		AnnotationKey: annotationKey,
		Value:         value,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get entities for string annotation %s=%s: %w", annotationKey, value, err)
	}

	// Convert string keys to common.Hash
	result := make([]common.Hash, 0, len(keys))
	for _, keyHex := range keys {
		result = append(result, common.HexToHash(keyHex))
	}

	return result, nil
}

// GetEntitiesForNumericAnnotationValue retrieves all entity keys that have a specific numeric annotation with the given value
func (e *SQLStore) GetEntitiesForNumericAnnotationValue(ctx context.Context, annotationKey string, value uint64) ([]common.Hash, error) {
	keys, err := e.GetQueries().GetEntitiesForNumericAnnotation(ctx, sqlitegolem.GetEntitiesForNumericAnnotationParams{
		AnnotationKey: annotationKey,
		Value:         int64(value),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get entities for numeric annotation %s=%d: %w", annotationKey, value, err)
	}

	// Convert string keys to common.Hash
	result := make([]common.Hash, 0, len(keys))
	for _, keyHex := range keys {
		result = append(result, common.HexToHash(keyHex))
	}

	return result, nil
}

// GetEntitiesOfOwner retrieves all entity keys owned by the specified address
func (e *SQLStore) GetEntitiesOfOwner(ctx context.Context, owner common.Address) ([]common.Hash, error) {
	keys, err := e.GetQueries().GetEntityKeysByOwner(ctx, owner.Hex())
	if err != nil {
		return nil, fmt.Errorf("failed to get entities for owner %s: %w", owner.Hex(), err)
	}

	// Convert string keys to common.Hash
	result := make([]common.Hash, 0, len(keys))
	for _, keyHex := range keys {
		result = append(result, common.HexToHash(keyHex))
	}

	return result, nil
}

// GetAllEntityKeys retrieves all entity keys from the database
func (e *SQLStore) GetAllEntityKeys(ctx context.Context) ([]common.Hash, error) {
	keys, err := e.GetQueries().GetAllEntityKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all entity keys: %w", err)
	}

	// Convert string keys to common.Hash
	result := make([]common.Hash, 0, len(keys))
	for _, keyHex := range keys {
		result = append(result, common.HexToHash(keyHex))
	}

	return result, nil
}

// GetEntityCount retrieves the total number of entities in the database
func (e *SQLStore) GetEntityCount(ctx context.Context) (uint64, error) {
	count, err := e.GetQueries().GetEntityCount(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get entity count: %w", err)
	}

	return uint64(count), nil
}

// GetEntityMetaData retrieves entity metadata from the database using a transaction
func (e *SQLStore) GetEntityMetaData(ctx context.Context, key common.Hash) (*entity.EntityMetaData, error) {
	// Begin a read-only transaction for consistency
	tx, err := e.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Safe to call even after commit

	txDB := sqlitegolem.New(tx)
	keyHex := key.Hex()

	// Get main entity data
	entityData, err := txDB.GetEntityMetadata(ctx, keyHex)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entity %s not found", keyHex)
		}
		return nil, fmt.Errorf("failed to get entity metadata: %w", err)
	}

	// Get string annotations
	stringAnnotRows, err := txDB.GetEntityStringAnnotations(ctx, keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to get string annotations: %w", err)
	}

	// Get numeric annotations
	numericAnnotRows, err := txDB.GetEntityNumericAnnotations(ctx, keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to get numeric annotations: %w", err)
	}

	// Convert to entity.EntityMetaData structure
	metadata := &entity.EntityMetaData{
		ExpiresAtBlock:     uint64(entityData.ExpiresAt),
		StringAnnotations:  make([]entity.StringAnnotation, len(stringAnnotRows)),
		NumericAnnotations: make([]entity.NumericAnnotation, len(numericAnnotRows)),
		Owner:              common.HexToAddress(entityData.OwnerAddress),
	}

	// Convert string annotations
	for i, row := range stringAnnotRows {
		metadata.StringAnnotations[i] = entity.StringAnnotation{
			Key:   row.AnnotationKey,
			Value: row.Value,
		}
	}

	// Convert numeric annotations
	for i, row := range numericAnnotRows {
		metadata.NumericAnnotations[i] = entity.NumericAnnotation{
			Key:   row.AnnotationKey,
			Value: uint64(row.Value),
		}
	}

	// Commit the transaction (read-only, but ensures consistency)
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return metadata, nil
}

func (e *SQLStore) SnapSyncToBlock(
	ctx context.Context,
	networkID string,
	blockNumber uint64,
	blockHash common.Hash,
	entities iter.Seq2[
		*struct {
			Key      common.Hash
			Metadata entity.EntityMetaData
			Payload  []byte
		},
		error,
	],
) (err error) {
	log.Info("snap syncing to block start", "blockNumber", blockNumber, "blockHash", blockHash.Hex())
	defer log.Info("snap syncing to block end", "blockNumber", blockNumber, "blockHash", blockHash.Hex())

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	txDB := sqlitegolem.New(tx)

	// Ensure single network constraint for snap sync
	hasNetwork, err := txDB.HasProcessingStatus(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to check if network exists: %w", err)
	}

	if !hasNetwork {
		// This is a new network, check if there are already other networks
		networkCount, err := txDB.CountNetworks(ctx)
		if err != nil {
			return fmt.Errorf("failed to count existing networks: %w", err)
		}

		if networkCount > 0 {
			return fmt.Errorf("cannot snap sync to network %s: database already contains %d network(s), only one network is allowed", networkID, networkCount)
		}

		// First network, need to insert initial processing status
		err = txDB.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
			Network:                  networkID,
			LastProcessedBlockNumber: int64(blockNumber),
			LastProcessedBlockHash:   blockHash.Hex(),
		})
		if err != nil {
			return fmt.Errorf("failed to insert initial processing status: %w", err)
		}
	}

	// Clear all existing entities, annotations for a clean snap sync
	err = txDB.DeleteAllEntities(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear entities: %w", err)
	}

	err = txDB.DeleteAllStringAnnotations(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear string annotations: %w", err)
	}

	err = txDB.DeleteAllNumericAnnotations(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear numeric annotations: %w", err)
	}

	// Insert all entities from the snapshot
	for entity, err := range entities {
		if err != nil {
			return fmt.Errorf("failed to get entity: %w", err)
		}

		// Insert the entity
		err = txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
			Key:                         entity.Key.Hex(),
			ExpiresAt:                   int64(entity.Metadata.ExpiresAtBlock),
			Payload:                     entity.Payload,
			OwnerAddress:                entity.Metadata.Owner.Hex(),
			CreatedAtBlock:              int64(entity.Metadata.CreatedAtBlock),
			LastModifiedAtBlock:         int64(entity.Metadata.LastModifiedAtBlock),
			TransactionIndexInBlock:     int64(entity.Metadata.TransactionIndex),
			OperationIndexInTransaction: int64(entity.Metadata.OperationIndex),
		})
		if err != nil {
			return fmt.Errorf("failed to insert entity %s: %w", entity.Key.Hex(), err)
		}

		// Insert string annotations
		for _, annotation := range entity.Metadata.StringAnnotations {
			err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
				EntityKey:     entity.Key.Hex(),
				AnnotationKey: annotation.Key,
				Value:         annotation.Value,
			})
			if err != nil {
				return fmt.Errorf("failed to insert string annotation for entity %s: %w", entity.Key.Hex(), err)
			}
		}

		// Insert numeric annotations
		for _, annotation := range entity.Metadata.NumericAnnotations {
			err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
				EntityKey:     entity.Key.Hex(),
				AnnotationKey: annotation.Key,
				Value:         int64(annotation.Value),
			})
			if err != nil {
				return fmt.Errorf("failed to insert numeric annotation for entity %s: %w", entity.Key.Hex(), err)
			}
		}
	}

	// Update processing status to the snap sync block
	err = txDB.UpdateProcessingStatus(ctx, sqlitegolem.UpdateProcessingStatusParams{
		Network:                  networkID,
		LastProcessedBlockNumber: int64(blockNumber),
		LastProcessedBlockHash:   blockHash.Hex(),
	})
	if err != nil {
		return fmt.Errorf("failed to update processing status: %w", err)
	}

	return tx.Commit()
}

// InsertBlock processes a single block from the WAL and inserts it into the database
func (e *SQLStore) InsertBlock(ctx context.Context, blockWal BlockWal, networkID string) (err error) {
	log.Info("processing block", "block", blockWal.BlockInfo.Number)
	defer log.Info("processing block end", "block", blockWal.BlockInfo.Number)

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	txDB := sqlitegolem.New(tx)

	// Ensure single network constraint: check if this would create a new network
	hasNetwork, err := txDB.HasProcessingStatus(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to check if network exists: %w", err)
	}

	if !hasNetwork {
		// This is a new network, check if there are already other networks
		networkCount, err := txDB.CountNetworks(ctx)
		if err != nil {
			return fmt.Errorf("failed to count existing networks: %w", err)
		}

		if networkCount > 0 {
			return fmt.Errorf("cannot add network %s: database already contains %d network(s), only one network is allowed", networkID, networkCount)
		}

		err = txDB.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
			Network:                  networkID,
			LastProcessedBlockNumber: int64(blockWal.BlockInfo.Number - 1),
			LastProcessedBlockHash:   blockWal.BlockInfo.ParentHash.Hex(),
		})
		if err != nil {
			return fmt.Errorf("failed to insert initial processing status: %w", err)
		}
	}

	log.Info("hasNetwork", "hasNetwork", hasNetwork)

	// Check if parent block hash matches the expected value from processing status
	if blockWal.BlockInfo.Number > 1 { // Skip check for genesis block
		processingStatus, err := txDB.GetProcessingStatus(ctx, networkID)
		if err != nil {
			return fmt.Errorf("failed to get processing status: %w", err)
		}

		expectedParentHash := processingStatus.LastProcessedBlockHash
		actualParentHash := blockWal.BlockInfo.ParentHash.Hex()

		if expectedParentHash != actualParentHash {
			return fmt.Errorf("parent block hash mismatch: expected %s, got %s", expectedParentHash, actualParentHash)
		}

		// Verify block number sequence
		expectedBlockNumber := processingStatus.LastProcessedBlockNumber + 1
		if int64(blockWal.BlockInfo.Number) != expectedBlockNumber {
			return fmt.Errorf("block number sequence error: expected %d, got %d", expectedBlockNumber, blockWal.BlockInfo.Number)
		}
	}

	for _, op := range blockWal.Operations {

		switch {
		case op.Create != nil:
			log.Info("create", "entity", op.Create.EntityKey.Hex())
			err = txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
				Key:                         op.Create.EntityKey.Hex(),
				ExpiresAt:                   int64(op.Create.ExpiresAtBlock),
				Payload:                     op.Create.Payload,
				OwnerAddress:                op.Create.Owner.Hex(),
				CreatedAtBlock:              int64(blockWal.BlockInfo.Number),
				LastModifiedAtBlock:         int64(blockWal.BlockInfo.Number),
				TransactionIndexInBlock:     int64(op.Create.TransactionIndex),
				OperationIndexInTransaction: int64(op.Create.OperationIndex),
			})
			if err != nil {
				return fmt.Errorf("failed to insert entity: %w", err)
			}

			for _, annotation := range op.Create.NumericAnnotations {
				err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
					EntityKey:     op.Create.EntityKey.Hex(),
					AnnotationKey: annotation.Key,
					Value:         int64(annotation.Value),
				})
				if err != nil {
					return fmt.Errorf("failed to insert numeric annotation: %w", err)
				}
			}

			for _, annotation := range op.Create.StringAnnotations {
				err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
					EntityKey:     op.Create.EntityKey.Hex(),
					AnnotationKey: annotation.Key,
					Value:         annotation.Value,
				})
				if err != nil {
					return fmt.Errorf("failed to insert string annotation: %w", err)
				}
			}
		case op.Update != nil:
			existingEntity, err := txDB.GetEntity(ctx, op.Update.EntityKey.Hex())
			if err != nil {
				return fmt.Errorf("failed to get existing entity: %w", err)
			}

			txDB.DeleteEntity(ctx, op.Update.EntityKey.Hex())
			txDB.DeleteNumericAnnotations(ctx, op.Update.EntityKey.Hex())
			txDB.DeleteStringAnnotations(ctx, op.Update.EntityKey.Hex())

			txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
				Key:                         op.Update.EntityKey.Hex(),
				ExpiresAt:                   int64(op.Update.ExpiresAtBlock),
				Payload:                     op.Update.Payload,
				OwnerAddress:                existingEntity.OwnerAddress,
				CreatedAtBlock:              existingEntity.CreatedAtBlock,
				LastModifiedAtBlock:         int64(blockWal.BlockInfo.Number),
				TransactionIndexInBlock:     int64(op.Update.TransactionIndex),
				OperationIndexInTransaction: int64(op.Update.OperationIndex),
			})

			for _, annotation := range op.Update.NumericAnnotations {
				err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
					EntityKey:     op.Update.EntityKey.Hex(),
					AnnotationKey: annotation.Key,
					Value:         int64(annotation.Value),
				})
				if err != nil {
					return fmt.Errorf("failed to insert numeric annotation: %w", err)
				}
			}

			for _, annotation := range op.Update.StringAnnotations {
				err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
					EntityKey:     op.Update.EntityKey.Hex(),
					AnnotationKey: annotation.Key,
					Value:         annotation.Value,
				})
				if err != nil {
					return fmt.Errorf("failed to insert string annotation: %w", err)
				}
			}
		case op.Delete != nil:
			err = txDB.DeleteEntity(ctx, op.Delete.Hex())
			if err != nil {
				return fmt.Errorf("failed to delete entity: %w", err)
			}

			err = txDB.DeleteNumericAnnotations(ctx, op.Delete.Hex())
			if err != nil {
				return fmt.Errorf("failed to delete numeric annotations: %w", err)
			}

			err = txDB.DeleteStringAnnotations(ctx, op.Delete.Hex())
			if err != nil {
				return fmt.Errorf("failed to delete string annotations: %w", err)
			}

		case op.Extend != nil:
			log.Info("extend BTL", "entity", op.Extend.EntityKey.Hex())

			// Update the entity with the new expiry time
			err = txDB.UpdateEntityExpiresAt(ctx, sqlitegolem.UpdateEntityExpiresAtParams{
				ExpiresAt:                   int64(op.Extend.NewExpiresAt),
				LastModifiedAtBlock:         int64(blockWal.BlockInfo.Number),
				TransactionIndexInBlock:     int64(op.Extend.TransactionIndex),
				OperationIndexInTransaction: int64(op.Extend.OperationIndex),
				Key:                         op.Extend.EntityKey.Hex(),
			})
			if err != nil {
				return fmt.Errorf("failed to extend entity BTL: %w", err)
			}
		}

		log.Info("operation", "operation", op)
	}

	err = txDB.UpdateProcessingStatus(ctx, sqlitegolem.UpdateProcessingStatusParams{
		Network:                  networkID,
		LastProcessedBlockNumber: int64(blockWal.BlockInfo.Number),
		LastProcessedBlockHash:   blockWal.BlockInfo.Hash.Hex(),
	})
	if err != nil {
		return fmt.Errorf("failed to insert processing status: %w", err)
	}

	return tx.Commit()
}

func (e *SQLStore) QueryEntities(ctx context.Context, query string, args ...any) ([]common.Hash, error) {
	log.Info("Executing query", "query", query, "args", args)

	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get entities for query: %s: %w", query, err)
	}
	defer rows.Close()

	keys := []common.Hash{}
	var key string
	for rows.Next() {
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to get entities for query: %s: %w", query, err)
		}
		keys = append(keys, common.HexToHash(key))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to get entities for query: %s: %w", query, err)
	}

	return keys, nil
}
