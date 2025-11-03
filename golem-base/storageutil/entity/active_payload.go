package entity

import (
	"github.com/ethereum/go-ethereum/common"
)

//go:generate go run ../../../rlp/rlpgen -type EntityMetaData -out gen_entity_meta_data_rlp.go

// EntityMetaData represents information about an entity that is currently active in the storage layer.
// This is what stored in the state.
// It contains a BTL (number of blocks) and a list of annotations.
// The Key of the entity is derived from the payload content and the transaction hash where the entity was created.

type EntityMetaData struct {
	ContentType         string              `json:"contentType"`
	ExpiresAtBlock      uint64              `json:"expiresAtBlock"`
	StringAnnotations   []StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations  []NumericAnnotation `json:"numericAnnotations"`
	Owner               common.Address      `json:"owner"`
	CreatedAtBlock      uint64              `json:"createdAtBlock"`
	LastModifiedAtBlock uint64              `json:"lastModifiedAtBlock"`
	TransactionIndex    uint64              `json:"transactionIndex"`
	OperationIndex      uint64              `json:"operationIndex"`
}

type StringAnnotation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type NumericAnnotation struct {
	Key   string `json:"key"`
	Value uint64 `json:"value"`
}
