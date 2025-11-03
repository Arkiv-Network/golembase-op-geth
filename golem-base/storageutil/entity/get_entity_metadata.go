package entity

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/klauspost/compress/zstd"
)

var EntityMetaDataSalt = []byte("arkivEntityMetaData")

var decoder, _ = zstd.NewReader(nil)

func GetEntityMetaData(access StateAccess, key common.Hash) (*EntityMetaData, error) {

	if !allentities.Contains(access, key) {
		return nil, fmt.Errorf("entity %s not found", key.Hex())
	}

	hash := crypto.Keccak256Hash(EntityMetaDataSalt, key[:])
	d := stateblob.GetBlob(access, hash)

	decoded, err := decoder.DecodeAll(d, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode compressed entity meta data: %w", err)
	}

	emd := EntityMetaData{}
	err = rlp.DecodeBytes(decoded, &emd)
	if err != nil {
		return nil, err
	}

	return &emd, nil

}
