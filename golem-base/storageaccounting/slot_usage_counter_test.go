package storageaccounting

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// mockStateAccess implements the StateAccess interface for testing
type mockStateAccess struct {
	state map[common.Address]map[common.Hash]common.Hash
}

func newMockStateAccess() *mockStateAccess {
	return &mockStateAccess{
		state: make(map[common.Address]map[common.Hash]common.Hash),
	}
}

func (m *mockStateAccess) GetState(address common.Address, key common.Hash) common.Hash {
	if addressState, exists := m.state[address]; exists {
		return addressState[key]
	}
	return common.Hash{}
}

func (m *mockStateAccess) SetState(address common.Address, key common.Hash, value common.Hash) common.Hash {
	if m.state[address] == nil {
		m.state[address] = make(map[common.Hash]common.Hash)
	}

	prev := m.state[address][key]
	m.state[address][key] = value
	return prev
}

func TestNewSlotUsageCounter(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	require.NotNil(t, counter)
	require.NotNil(t, counter.UsedSlots)
	require.Equal(t, mockAccess, counter.stateAccess)
	require.Empty(t, counter.UsedSlots)
}

func TestSlotUsageCounter_GetState(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	address := common.HexToAddress("0x1234")
	key := common.HexToHash("0x5678")
	expectedValue := common.HexToHash("0x9abc")

	// Set up mock state
	mockAccess.state[address] = map[common.Hash]common.Hash{
		key: expectedValue,
	}

	result := counter.GetState(address, key)
	require.Equal(t, expectedValue, result)
}

func TestSlotUsageCounter_SetState_NewValue(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	address := common.HexToAddress("0x1234")
	key := common.HexToHash("0x5678")
	value := common.HexToHash("0x9abc")

	prev := counter.SetState(address, key, value)

	// Should return empty hash as previous value (slot was empty)
	require.Equal(t, common.Hash{}, prev)

	// Should increment counter for this address
	usedSlots := counter.UsedSlots[address]
	require.NotNil(t, usedSlots)
	require.Equal(t, uint256.NewInt(1), usedSlots)

	// Verify state was actually set in underlying storage
	require.Equal(t, value, mockAccess.GetState(address, key))
}

func TestSlotUsageCounter_SetState_SameValue(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	address := common.HexToAddress("0x1234")
	key := common.HexToHash("0x5678")
	value := common.HexToHash("0x9abc")

	// Set initial value
	mockAccess.SetState(address, key, value)

	// Set same value again
	prev := counter.SetState(address, key, value)

	// Should return the same value as previous
	require.Equal(t, value, prev)

	// Counter should not be affected (no entry should exist)
	usedSlots := counter.UsedSlots[address]
	require.Nil(t, usedSlots)
}

func TestSlotUsageCounter_SetState_ClearValue(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	address := common.HexToAddress("0x1234")
	key := common.HexToHash("0x5678")
	value := common.HexToHash("0x9abc")

	// Set initial value
	mockAccess.SetState(address, key, value)

	// Clear the value (set to empty hash)
	prev := counter.SetState(address, key, common.Hash{})

	// Should return the previous value
	require.Equal(t, value, prev)

	// Should decrement counter (from 0 to -1)
	usedSlots := counter.UsedSlots[address]
	require.NotNil(t, usedSlots)
	require.Equal(t, uint256.NewInt(0).Sub(uint256.NewInt(0), uint256.NewInt(1)), usedSlots)

	// Verify state was cleared in underlying storage
	require.Equal(t, common.Hash{}, mockAccess.GetState(address, key))
}

func TestSlotUsageCounter_SetState_MultipleOperations(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	address := common.HexToAddress("0x1234")
	key1 := common.HexToHash("0x5678")
	key2 := common.HexToHash("0x9abc")
	value1 := common.HexToHash("0xdef0")
	value2 := common.HexToHash("0x1234")

	// Set two different keys
	counter.SetState(address, key1, value1)
	counter.SetState(address, key2, value2)

	// Counter should be 2
	usedSlots := counter.UsedSlots[address]
	require.NotNil(t, usedSlots)
	require.Equal(t, uint256.NewInt(2), usedSlots)

	// Clear one key
	counter.SetState(address, key1, common.Hash{})

	// Counter should be 1
	require.Equal(t, uint256.NewInt(1), usedSlots)

	// Clear the other key
	counter.SetState(address, key2, common.Hash{})

	// Counter should be 0
	require.Equal(t, uint256.NewInt(0), usedSlots)
}

func TestSlotUsageCounter_UpdateUsedSlotsForGolemBase(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	// Set up initial stored counter value
	initialStoredValue := uint256.NewInt(10)
	mockAccess.SetState(address.ArkivProcessorAddress, UsedSlotsKey, initialStoredValue.Bytes32())

	// Add some usage to the counter
	counter.UsedSlots[address.ArkivProcessorAddress] = uint256.NewInt(5)

	counter.UpdateUsedSlotsForGolemBase()

	// Should have updated the stored value (10 + 5 = 15)
	expectedTotal := uint256.NewInt(15)
	storedValue := mockAccess.GetState(address.ArkivProcessorAddress, UsedSlotsKey)
	storedInt := uint256.NewInt(0)
	storedInt.SetBytes32(storedValue.Bytes())
	require.Equal(t, expectedTotal, storedInt)

	// Counter should be cleared
	usedSlots := counter.UsedSlots[address.ArkivProcessorAddress]
	require.NotNil(t, usedSlots)
	require.True(t, usedSlots.IsZero(), "Counter should be zero after being cleared")
}

func TestSlotUsageCounter_UpdateUsedSlotsForGolemBase_NoInitialValue(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	// Add some usage to the counter (no initial stored value)
	counter.UsedSlots[address.ArkivProcessorAddress] = uint256.NewInt(3)

	counter.UpdateUsedSlotsForGolemBase()

	// Should have stored the counter value (0 + 3 = 3)
	expectedTotal := uint256.NewInt(3)
	storedValue := mockAccess.GetState(address.ArkivProcessorAddress, UsedSlotsKey)
	storedInt := uint256.NewInt(0)
	storedInt.SetBytes32(storedValue.Bytes())
	require.Equal(t, expectedTotal, storedInt)

	// Counter should be cleared
	usedSlots := counter.UsedSlots[address.ArkivProcessorAddress]
	require.NotNil(t, usedSlots)
	require.True(t, usedSlots.IsZero(), "Counter should be zero after being cleared")
}

func TestSlotUsageCounter_UpdateUsedSlotsForGolemBase_NoCounter(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	// Set up initial stored counter value
	initialStoredValue := uint256.NewInt(7)
	mockAccess.SetState(address.ArkivProcessorAddress, UsedSlotsKey, initialStoredValue.Bytes32())

	// No counter entry for golem address
	counter.UpdateUsedSlotsForGolemBase()

	// Should keep the initial stored value (7 + 0 = 7)
	storedValue := mockAccess.GetState(address.ArkivProcessorAddress, UsedSlotsKey)
	storedInt := uint256.NewInt(0)
	storedInt.SetBytes32(storedValue.Bytes())
	require.Equal(t, initialStoredValue, storedInt)

	// Counter should be created and cleared
	usedSlots := counter.UsedSlots[address.ArkivProcessorAddress]
	require.NotNil(t, usedSlots)
	require.Equal(t, uint256.NewInt(0), usedSlots)
}

func TestUsedSlotsKey(t *testing.T) {
	// Test that UsedSlotsKey is correctly computed and deterministic
	// The actual value will depend on the crypto.Keccak256Hash implementation
	require.NotEqual(t, common.Hash{}, UsedSlotsKey)
	require.Equal(t, 32, len(UsedSlotsKey.Bytes()))
}

func TestSlotUsageCounter_SetState_MultipleAddresses(t *testing.T) {
	mockAccess := newMockStateAccess()
	counter := NewSlotUsageCounter(mockAccess)

	address1 := common.HexToAddress("0x1111")
	address2 := common.HexToAddress("0x2222")
	key := common.HexToHash("0x5678")
	value := common.HexToHash("0x9abc")

	// Set values for different addresses
	counter.SetState(address1, key, value)
	counter.SetState(address2, key, value)

	// Both addresses should have counters
	require.Equal(t, uint256.NewInt(1), counter.UsedSlots[address1])
	require.Equal(t, uint256.NewInt(1), counter.UsedSlots[address2])

	// Verify independence
	counter.SetState(address1, key, common.Hash{})
	require.Equal(t, uint256.NewInt(0), counter.UsedSlots[address1])
	require.Equal(t, uint256.NewInt(1), counter.UsedSlots[address2])
}
