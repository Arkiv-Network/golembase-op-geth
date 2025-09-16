package storagetx

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

//go:generate go run ../../rlp/rlpgen -type StorageTransaction -out gen_storage_transaction_rlp.go

// GolemBaseStorageEntityCreated is the event signature for entity creation logs.
var GolemBaseStorageEntityCreated = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityCreated(uint256,uint256)"))

// GolemBaseStorageEntityDeleted is the event signature for entity deletion logs.
var GolemBaseStorageEntityDeleted = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityDeleted(uint256)"))

// GolemBaseStorageEntityUpdated is the event signature for entity update logs.
var GolemBaseStorageEntityUpdated = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityUpdated(uint256,uint256)"))

// GolemBaseStorageEntityBTLExtended is the event signature for extending BTL of an entity.
var GolemBaseStorageEntityBTLExtended = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityBTLExtended(uint256,uint256,uint256)"))

// StorageTransaction represents a transaction that can be applied to the storage layer.
// It contains a list of Create operations, a list of Update operations and a list of Delete operations.
//
// Semantics of the transaction operations are as follows:
//   - Create: adds new entities to the storage layer. Each entity has a BTL (number of blocks), a payload and a list of annotations. The Key of the entity is derived from the payload content, the transaction hash where the entity was created and the index of the create operation in the transaction.
//   - Update: updates existing entities. Each entity has a key, a BTL (number of blocks), a payload and a list of annotations. If the entity does not exist, the operation fails, failing the whole transaction.
//   - Delete: removes entities from the storage layer. If the entity does not exist, the operation fails, failing back the whole transaction.
//
// The transaction is atomic, meaning that all operations are applied or none are.
//
// Annotations are key-value pairs where the key is a string and the value is either a string or a number.
// The key-value pairs are used to build indexes and to query the storage layer.
// Same key can have both string and numeric annotation, but not multiple values of the same type.
type StorageTransaction struct {
	Create []Create      `json:"create"`
	Update []Update      `json:"update"`
	Delete []common.Hash `json:"delete"`
	Extend []ExtendBTL   `json:"extend"`
}

func (tx *StorageTransaction) Validate() error {

	for i, create := range tx.Create {
		if create.BTL == 0 {
			return fmt.Errorf("create BTL is 0")
		}

		seenStringAnnotations := make(map[string]bool)
		seenNumericAnnotations := make(map[string]bool)

		// Validate the annotation identifiers
		for _, annotation := range create.StringAnnotations {
			if !entity.AnnotationIdentRegexCompiled.MatchString(annotation.Key) {
				return fmt.Errorf("invalid annotation identifier (must match `%s`): %s",
					entity.AnnotationIdentRegexCompiled.String(),
					annotation.Key,
				)
			}
			if seenStringAnnotations[annotation.Key] {
				return fmt.Errorf("create[%d] string annotation key %s is duplicated", i, annotation.Key)
			}

			seenStringAnnotations[annotation.Key] = true

		}
		for _, annotation := range create.NumericAnnotations {
			if !entity.AnnotationIdentRegexCompiled.MatchString(annotation.Key) {
				return fmt.Errorf("invalid annotation identifier (must match `%s`): %s",
					entity.AnnotationIdentRegexCompiled.String(),
					annotation.Key,
				)
			}
			if seenNumericAnnotations[annotation.Key] {
				return fmt.Errorf("create[%d] numeric annotation key %s is duplicated", i, annotation.Key)
			}
			seenNumericAnnotations[annotation.Key] = true
		}

	}

	for i, update := range tx.Update {
		if update.BTL == 0 {
			return fmt.Errorf("update[%d] BTL is 0", i)
		}

		seenStringAnnotations := make(map[string]bool)
		seenNumericAnnotations := make(map[string]bool)

		for _, annotation := range update.StringAnnotations {
			if !entity.AnnotationIdentRegexCompiled.MatchString(annotation.Key) {
				return fmt.Errorf("invalid annotation identifier (must match `%s`): %s",
					entity.AnnotationIdentRegexCompiled.String(),
					annotation.Key,
				)
			}
			if seenStringAnnotations[annotation.Key] {
				return fmt.Errorf("update[%d] string annotation key %s is duplicated", i, annotation.Key)
			}
			seenStringAnnotations[annotation.Key] = true
		}
		for _, annotation := range update.NumericAnnotations {
			if !entity.AnnotationIdentRegexCompiled.MatchString(annotation.Key) {
				return fmt.Errorf("invalid annotation identifier (must match `%s`): %s",
					entity.AnnotationIdentRegexCompiled.String(),
					annotation.Key,
				)
			}
			if seenNumericAnnotations[annotation.Key] {
				return fmt.Errorf("update[%d] numeric annotation key %s is duplicated", i, annotation.Key)
			}
			seenNumericAnnotations[annotation.Key] = true
		}

	}

	for i, extend := range tx.Extend {
		if extend.NumberOfBlocks == 0 {
			return fmt.Errorf("extend[%d] number of blocks is 0", i)
		}
	}

	return nil

}

type Create struct {
	BTL                uint64                     `json:"btl"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
}

type Update struct {
	EntityKey          common.Hash                `json:"entityKey"`
	BTL                uint64                     `json:"btl"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
}

type ExtendBTL struct {
	EntityKey      common.Hash `json:"entityKey"`
	NumberOfBlocks uint64      `json:"numberOfBlocks"`
}

func (tx *StorageTransaction) Run(blockNumber uint64, txHash common.Hash, txIx int, sender common.Address, access storageutil.StateAccess) (_ []*types.Log, err error) {

	defer func() {
		if err != nil {
			log.Error("failed to run storage transaction", "error", err)
		}
	}()

	err = tx.Validate()
	if err != nil {
		return nil, fmt.Errorf("failed to validate storage transaction: %w", err)
	}

	logs := []*types.Log{}

	storeEntity := func(key common.Hash, ap *entity.EntityMetaData, payload []byte, emitLogs bool) error {

		err := entity.Store(access, key, sender, *ap, payload)
		if err != nil {
			return fmt.Errorf("failed to store entity: %w", err)
		}

		if emitLogs {
			expiresAtBlockNumberBig := uint256.NewInt(ap.ExpiresAtBlock)

			data := make([]byte, 32)
			expiresAtBlockNumberBig.PutUint256(data[:32])

			// create the log for the created entity
			log := &types.Log{
				Address:     address.GolemBaseStorageProcessorAddress,
				Topics:      []common.Hash{GolemBaseStorageEntityCreated, key},
				Data:        data,
				BlockNumber: blockNumber,
			}
			logs = append(logs, log)
		}

		return nil

	}

	for opIx, create := range tx.Create {

		if create.BTL == 0 {
			return nil, fmt.Errorf("create BTL is 0 for create %d", opIx)
		}

		// Convert i to a big integer and pad to 32 bytes
		bigI := big.NewInt(int64(opIx))
		paddedI := common.LeftPadBytes(bigI.Bytes(), 32)

		key := crypto.Keccak256Hash(txHash.Bytes(), create.Payload, paddedI)

		ap := &entity.EntityMetaData{
			Owner:               sender,
			ExpiresAtBlock:      blockNumber + create.BTL,
			StringAnnotations:   create.StringAnnotations,
			NumericAnnotations:  create.NumericAnnotations,
			CreatedAtBlock:      blockNumber,
			LastModifiedAtBlock: blockNumber,
			OperationIndex:      uint64(opIx),
			TransactionIndex:    uint64(txIx),
		}

		err := storeEntity(key, ap, create.Payload, true)

		if err != nil {
			return nil, err
		}

	}

	deleteEntity := func(toDelete common.Hash, emitLogs bool) error {

		err := entity.Delete(access, toDelete)
		if err != nil {
			return fmt.Errorf("failed to delete entity: %w", err)
		}

		if emitLogs {

			// create the log for the created entity
			log := &types.Log{
				Address:     address.GolemBaseStorageProcessorAddress,
				Topics:      []common.Hash{GolemBaseStorageEntityDeleted, toDelete},
				Data:        []byte{},
				BlockNumber: blockNumber,
			}

			logs = append(logs, log)
		}

		return nil

	}

	for _, toDelete := range tx.Delete {
		metaData, err := entity.GetEntityMetaData(access, toDelete)
		if err != nil {
			return nil, fmt.Errorf("failed to get entity meta data for delete %s: %w", toDelete.Hex(), err)
		}

		if metaData.Owner != sender {
			return nil, fmt.Errorf("failed to delete entity %s: %s is not the owner", toDelete.Hex(), sender.Hex())
		}

		err = deleteEntity(toDelete, true)
		if err != nil {
			return nil, err
		}
	}

	for opIx, update := range tx.Update {

		if update.BTL == 0 {
			return nil, fmt.Errorf("update BTL is 0 for entity %s", update.EntityKey.Hex())
		}

		oldMetaData, err := entity.GetEntityMetaData(access, update.EntityKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get entity meta data for update %s: %w", update.EntityKey.Hex(), err)
		}

		if oldMetaData.Owner != sender {
			return nil, fmt.Errorf("failed to update entity %s: %s is not the owner", update.EntityKey.Hex(), sender.Hex())
		}

		err = deleteEntity(update.EntityKey, false)
		if err != nil {
			return nil, err
		}

		ap := &entity.EntityMetaData{
			ExpiresAtBlock:      blockNumber + update.BTL,
			StringAnnotations:   update.StringAnnotations,
			NumericAnnotations:  update.NumericAnnotations,
			Owner:               oldMetaData.Owner,
			CreatedAtBlock:      oldMetaData.CreatedAtBlock,
			LastModifiedAtBlock: blockNumber,
			OperationIndex:      uint64(opIx),
			TransactionIndex:    uint64(txIx),
		}

		err = storeEntity(update.EntityKey, ap, update.Payload, false)

		if err != nil {
			return nil, err
		}

		expiresAtBlockNumberBig := uint256.NewInt(ap.ExpiresAtBlock)
		data := make([]byte, 32)
		expiresAtBlockNumberBig.PutUint256(data[:32])

		logs = append(logs, &types.Log{
			Address:     address.GolemBaseStorageProcessorAddress,
			Topics:      []common.Hash{GolemBaseStorageEntityUpdated, update.EntityKey},
			Data:        data,
			BlockNumber: blockNumber,
		})

	}

	for _, extend := range tx.Extend {
		newExpiresAtBlock, err := entity.ExtendBTL(access, extend.EntityKey, extend.NumberOfBlocks)
		if err != nil {
			return nil, fmt.Errorf("failed to extend BTL of entity %s: %w", extend.EntityKey.Hex(), err)
		}

		oldExpiresAtBlock := newExpiresAtBlock - extend.NumberOfBlocks

		oldExpiresAtBlockBig := uint256.NewInt(oldExpiresAtBlock)
		newExpiresAtBlockBig := uint256.NewInt(newExpiresAtBlock)

		data := make([]byte, 64)

		oldExpiresAtBlockBig.PutUint256(data[:32])
		newExpiresAtBlockBig.PutUint256(data[32:])

		logs = append(logs, &types.Log{
			Address:     address.GolemBaseStorageProcessorAddress,
			Topics:      []common.Hash{GolemBaseStorageEntityBTLExtended, extend.EntityKey},
			Data:        data,
			BlockNumber: blockNumber,
		})
	}

	return logs, nil
}

func ExecuteTransaction(d []byte, blockNumber uint64, txHash common.Hash, txIx int, sender common.Address, access storageutil.StateAccess) ([]*types.Log, error) {
	tx := &StorageTransaction{}
	err := rlp.DecodeBytes(d, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode storage transaction: %w", err)
	}

	st := storageaccounting.NewSlotUsageCounter(access)

	logs, err := tx.Run(blockNumber, txHash, txIx, sender, st)
	if err != nil {
		log.Error("Failed to run storage transaction", "error", err)
		return nil, fmt.Errorf("failed to run storage transaction: %w", err)
	}

	st.UpdateUsedSlotsForGolemBase()

	return logs, nil
}
