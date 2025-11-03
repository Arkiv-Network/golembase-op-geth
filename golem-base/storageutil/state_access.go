package storageutil

import (
	"github.com/ethereum/go-ethereum/common"
)

type StateAccess interface {
	GetState(common.Address, common.Hash) common.Hash
	SetState(common.Address, common.Hash, common.Hash) common.Hash
}
