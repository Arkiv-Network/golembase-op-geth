package entityexpiration

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/holiman/uint256"
)

type StateAccess = storageutil.StateAccess

var BlockExpirationSalt = []byte("arkivExpiresAtBlock")

func AddToEntitiesToExpireAtBlock(access StateAccess, blockNumber uint64, entityKey common.Hash) error {
	expiresAtBlockNumberBig := uint256.NewInt(blockNumber)
	expiredEntityKey := crypto.Keccak256Hash(BlockExpirationSalt, expiresAtBlockNumberBig.Bytes())
	err := keyset.AddValue(access, expiredEntityKey, entityKey)
	if err != nil {
		return fmt.Errorf("failed to append to key list: %w", err)
	}

	return nil
}
