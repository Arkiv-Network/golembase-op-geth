package entity

import (
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

//go:generate go run ../../../rlp/rlpgen -type EncodableEntityMetaData -out gen_entity_meta_data_rlp.go

// EntityMetaData represents information about an entity that is currently active in the storage layer.
// This is what stored in the state.
// It contains a BTL (number of blocks) and a list of annotations.
// The Key of the entity is derived from the payload content and the transaction hash where the entity was created.

type EntityMetaData struct {
	ExpiresAtBlock      uint64              `json:"expiresAtBlock"`
	StringAnnotations   []StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations  []NumericAnnotation `json:"numericAnnotations"`
	Owner               common.Address      `json:"owner"`
	CreatedAtBlock      uint64              `json:"createdAtBlock"`
	LastModifiedAtBlock uint64              `json:"lastModifiedAtBlock"`
	TransactionIndex    uint64              `json:"transactionIndex"`
	OperationIndex      uint64              `json:"operationIndex"`
}

type EncodableEntityMetaData struct {
	ExpiresAtBlock     uint64
	StringAnnotations  []StringAnnotation
	NumericAnnotations []NumericAnnotation
	Owner              common.Address
	BlockInfo          uint256.Int
}

func (m *EntityMetaData) EncodeRLP(w io.Writer) error {

	blockInfo := uint256.NewInt(m.OperationIndex)
	blockInfo = blockInfo.Lsh(blockInfo, 64)
	blockInfo = blockInfo.AddUint64(blockInfo, m.TransactionIndex)
	blockInfo = blockInfo.Lsh(blockInfo, 64)
	blockInfo = blockInfo.AddUint64(blockInfo, m.LastModifiedAtBlock)
	blockInfo = blockInfo.Lsh(blockInfo, 64)
	blockInfo = blockInfo.AddUint64(blockInfo, m.CreatedAtBlock)

	encodable := EncodableEntityMetaData{
		ExpiresAtBlock:     m.ExpiresAtBlock,
		StringAnnotations:  m.StringAnnotations,
		NumericAnnotations: m.NumericAnnotations,
		Owner:              m.Owner,
		BlockInfo:          *blockInfo,
	}

	return rlp.Encode(w, &encodable)
}

func (m *EntityMetaData) DecodeRLP(s *rlp.Stream) error {

	encodable := EncodableEntityMetaData{}
	s.Decode(&encodable)

	blockInfo := &encodable.BlockInfo

	created := blockInfo.Uint64()
	blockInfo = blockInfo.Rsh(blockInfo, 64)
	lastModified := blockInfo.Uint64()
	blockInfo = blockInfo.Rsh(blockInfo, 64)
	transactionIndex := blockInfo.Uint64()
	blockInfo = blockInfo.Rsh(blockInfo, 64)
	operationIndex := blockInfo.Uint64()

	*m = EntityMetaData{
		ExpiresAtBlock:      encodable.ExpiresAtBlock,
		StringAnnotations:   encodable.StringAnnotations,
		NumericAnnotations:  encodable.NumericAnnotations,
		Owner:               encodable.Owner,
		CreatedAtBlock:      created,
		LastModifiedAtBlock: lastModified,
		TransactionIndex:    transactionIndex,
		OperationIndex:      operationIndex,
	}

	return nil
}

type StringAnnotation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type NumericAnnotation struct {
	Key   string `json:"key"`
	Value uint64 `json:"value"`
}
