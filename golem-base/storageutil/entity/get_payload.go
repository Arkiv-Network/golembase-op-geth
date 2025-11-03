package entity

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
)

func GetPayload(access StateAccess, key common.Hash) ([]byte, error) {
	hash := crypto.Keccak256Hash(PayloadSalt, key[:])
	d := stateblob.GetBlob(access, hash)
	decoded, err := decoder.DecodeAll(d, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode compressed payload: %w", err)
	}
	return decoded, nil
}

func GetCompressedPayload(access StateAccess, key common.Hash) []byte {
	hash := crypto.Keccak256Hash(PayloadSalt, key[:])
	d := stateblob.GetBlob(access, hash)
	return d
}
