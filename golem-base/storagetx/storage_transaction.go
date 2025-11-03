package storagetx

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
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

func (tx *StorageTransaction) ConvertToArkiv() *ArkivTransaction {
	atx := ArkivTransaction{}
	for _, create := range tx.Create {
		atx.Create = append(atx.Create, ArkivCreate{
			BTL:                create.BTL,
			ContentType:        "application/octet-stream",
			Payload:            create.Payload,
			StringAnnotations:  create.StringAnnotations,
			NumericAnnotations: create.NumericAnnotations,
		})
	}
	for _, update := range tx.Update {
		atx.Update = append(atx.Update, ArkivUpdate{
			EntityKey:          update.EntityKey,
			BTL:                update.BTL,
			ContentType:        "application/octet-stream",
			Payload:            update.Payload,
			StringAnnotations:  update.StringAnnotations,
			NumericAnnotations: update.NumericAnnotations,
		})
	}
	atx.Delete = tx.Delete
	atx.Extend = tx.Extend

	return &atx
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

func addressToHash(a common.Address) common.Hash {
	h := common.Hash{}
	copy(h[12:], a[:])
	return h
}

func (tx *StorageTransaction) Validate() error {
	return tx.ConvertToArkiv().Validate()
}

func ExecuteTransaction(d []byte, blockNumber uint64, txHash common.Hash, txIx int, sender common.Address, access storageutil.StateAccess) ([]*types.Log, error) {
	tx := &StorageTransaction{}
	err := rlp.DecodeBytes(d, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode storage transaction: %w", err)
	}

	st := storageaccounting.NewSlotUsageCounter(access)

	logs, err := tx.ConvertToArkiv().Run(blockNumber, txHash, txIx, sender, st)
	if err != nil {
		log.Error("Failed to run storage transaction", "error", err)
		return nil, fmt.Errorf("failed to run storage transaction: %w", err)
	}

	st.UpdateUsedSlotsForGolemBase()

	return logs, nil
}
