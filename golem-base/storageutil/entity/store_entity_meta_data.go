package entity

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
	"github.com/ethereum/go-ethereum/rlp"
)

func StoreEntityMetaData(access StateAccess, key common.Hash, emd EntityMetaData) error {
	hash := crypto.Keccak256Hash(EntityMetaDataSalt, key[:])

	buf := new(bytes.Buffer)
	err := rlp.Encode(buf, &emd)
	if err != nil {
		return fmt.Errorf("failed to encode entity meta data: %w", err)
	}

	compressed := encoder.EncodeAll(buf.Bytes(), nil)

	stateblob.SetBlob(access, hash, compressed)
	return nil
}
