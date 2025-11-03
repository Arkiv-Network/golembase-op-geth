package storageaccounting

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/holiman/uint256"
)

var UsedSlotsKey = crypto.Keccak256Hash([]byte("arkivUsedSlots"))

type SlotUsageCounter struct {
	UsedSlots   map[common.Address]*uint256.Int
	stateAccess storageutil.StateAccess
}

func NewSlotUsageCounter(stateAccess storageutil.StateAccess) *SlotUsageCounter {
	return &SlotUsageCounter{
		UsedSlots:   make(map[common.Address]*uint256.Int),
		stateAccess: stateAccess,
	}
}

func (c *SlotUsageCounter) GetState(address common.Address, key common.Hash) common.Hash {
	return c.stateAccess.GetState(address, key)
}

func (c *SlotUsageCounter) SetState(address common.Address, key common.Hash, value common.Hash) common.Hash {

	prev := c.stateAccess.SetState(address, key, value)

	// nothing to do if the value is the same
	if prev == value {
		return prev
	}

	counter := c.UsedSlots[address]
	if counter == nil {
		counter = uint256.NewInt(0)
		c.UsedSlots[address] = counter
	}

	switch {
	case prev == (common.Hash{}) && value != (common.Hash{}):
		counter.Add(counter, uint256.NewInt(1))
	case prev != (common.Hash{}) && value == (common.Hash{}):
		counter.Sub(counter, uint256.NewInt(1))
	}

	return prev
}

func (c *SlotUsageCounter) UpdateUsedSlotsForGolemBase() {
	storedSlotsCounter := uint256.NewInt(0)
	storedSlotsCounter.SetBytes32(c.stateAccess.GetState(address.ArkivProcessorAddress, UsedSlotsKey).Bytes())

	counter := c.UsedSlots[address.ArkivProcessorAddress]
	if counter == nil {
		counter = uint256.NewInt(0)
		c.UsedSlots[address.ArkivProcessorAddress] = counter
	}

	storedSlotsCounter.Add(storedSlotsCounter, counter)

	c.stateAccess.SetState(address.ArkivProcessorAddress, UsedSlotsKey, storedSlotsCounter.Bytes32())
	counter.SetUint64(0)
}
