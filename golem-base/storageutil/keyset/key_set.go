// Package keyset provides a set data structure implementation for the Ethereum state.
// This is a Go implementation of the same data structure pattern used in OpenZeppelin's
// EnumerableSet (https://github.com/OpenZeppelin/openzeppelin-contracts/blob/master/contracts/utils/structs/EnumerableSet.sol)
// It provides O(1) operations for adding, removing, and checking membership in a set,
// while also maintaining the ability to enumerate elements.
package keyset

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset/array"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset/hashmap"
	"github.com/holiman/uint256"
)

type StateAccess = storageutil.StateAccess

var zeroHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
var oneUint256 = new(uint256.Int).SetUint64(1)
var MapKeyPrefix = []byte("arkivKeysetMap")

// ContainsValue checks if the given value exists in the set identified by setKey.
// It returns true if the value is present in the set, false otherwise.
func ContainsValue(db StateAccess, setKey common.Hash, value common.Hash) bool {
	m := hashmap.NewMap(db, MapKeyPrefix, setKey[:])
	return m.Get(value) != zeroHash
}

// AddValue adds a value to the set identified by setKey.
// If the value already exists in the set, it does nothing.
// Returns an error if there are any issues during the operation.
func AddValue(db StateAccess, setKey common.Hash, value common.Hash) error {

	array := array.NewArray(db, setKey)
	m := hashmap.NewMap(db, MapKeyPrefix, setKey[:])

	// if the value is already in the set, do nothing
	if ContainsValue(db, setKey, value) {
		return nil
	}

	array.Append(value)
	m.Set(value, array.Size().Bytes32())

	return nil

}

// RemoveValue removes a value from the set identified by setKey.
// It does nothing if the value is not in the set.
// For non-empty sets, it moves the last element to the position of the removed element
// to maintain a compact array representation.
// Returns an error if there are any issues during the operation.
func RemoveValue(db StateAccess, setKey common.Hash, value common.Hash) error {

	array := array.NewArray(db, setKey)
	m := hashmap.NewMap(db, MapKeyPrefix, setKey[:])

	if !ContainsValue(db, setKey, value) {
		return nil
	}

	elementIndex := new(uint256.Int).SetBytes32(m.Get(value).Bytes())
	elementIndex.Sub(elementIndex, oneUint256)

	oldSize := array.Size()

	lastElementIndex := new(uint256.Int).Set(oldSize)
	lastElementIndex.Sub(lastElementIndex, oneUint256)
	lastElementValue, err := array.Get(lastElementIndex)
	if err != nil {
		return fmt.Errorf("failed to get last element: %w", err)
	}

	m.Set(value, zeroHash)

	if lastElementIndex.Cmp(elementIndex) != 0 {
		array.Set(elementIndex, lastElementValue)
		elementIndexPlusOne := new(uint256.Int).Set(elementIndex)
		elementIndexPlusOne.Add(elementIndexPlusOne, oneUint256)
		m.Set(lastElementValue, elementIndexPlusOne.Bytes32())
	}

	err = array.RemoveLast()
	if err != nil {
		return fmt.Errorf("failed to remove last element: %w", err)
	}

	return nil

}

// Size returns the number of elements in the set as a uint256
func Size(db StateAccess, setKey common.Hash) *uint256.Int {
	array := array.NewArray(db, setKey)
	return array.Size()
}

// Clear removes all elements from the set.
// It iterates through all values in the set and clears their mappings,
// then resets the set's size to zero.
// This operation is O(n) where n is the number of elements in the set.
func Clear(db StateAccess, setKey common.Hash) {
	array := array.NewArray(db, setKey)
	m := hashmap.NewMap(db, MapKeyPrefix, setKey[:])

	for v := range array.Iterate {
		m.Set(v, zeroHash)
	}
	array.Clear()
}

func Iterate(db StateAccess, setKey common.Hash) func(yield func(value common.Hash) bool) {
	array := array.NewArray(db, setKey)
	return array.Iterate
}
