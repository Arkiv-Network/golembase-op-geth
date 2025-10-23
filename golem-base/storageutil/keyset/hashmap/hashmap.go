package hashmap

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
)

type Map struct {
	db   storageutil.StateAccess
	salt []byte
}

func NewMap(db storageutil.StateAccess, salts ...[]byte) *Map {
	combinedSalt := []byte{}
	for _, s := range salts {
		combinedSalt = append(combinedSalt, s...)
	}
	return &Map{db: db, salt: combinedSalt}
}

func (m *Map) Get(key common.Hash) common.Hash {
	hash := crypto.Keccak256Hash(m.salt, key.Bytes())
	return m.db.GetState(address.ArkivProcessorAddress, hash)
}

func (m *Map) Set(key common.Hash, value common.Hash) {
	hash := crypto.Keccak256Hash(m.salt, key.Bytes())
	m.db.SetState(address.ArkivProcessorAddress, hash, value)
}
