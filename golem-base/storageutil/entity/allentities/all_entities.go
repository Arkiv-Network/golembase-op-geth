// Package allentities provides access to the global list of all entities in the Golem system.
//
// This package maintains a global registry of entity hashes that can be used to track
// and enumerate all entities that exist in the system. It provides functionality to add,
// remove, and iterate through all registered entities.
//
// The implementation uses the keyset package to provide efficient O(1) operations for
// adding and removing entities, while maintaining the ability to enumerate all entities.

package allentities

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
)

type StateAccess = storageutil.StateAccess

// AllEntitiesKey is the storage key that identifies the global registry of all entities.
// This key is used as the set identifier when interacting with the keyset package.
// It's derived from a unique string to avoid collisions with other storage keys.
var AllEntitiesKey = crypto.Keccak256Hash([]byte("arkivAllEntities"))

// AddEntity adds a new entity hash to the global registry.
func AddEntity(db StateAccess, hash common.Hash) error {
	return keyset.AddValue(db, AllEntitiesKey, hash)
}

// RemoveEntity removes an entity hash from the global registry.
func RemoveEntity(db StateAccess, hash common.Hash) error {
	return keyset.RemoveValue(db, AllEntitiesKey, hash)
}

// Iterate provides a function that can be used to iterate over all entity hashes in the registry.
func Iterate(db StateAccess) func(yield func(hash common.Hash) bool) {
	return keyset.Iterate(db, AllEntitiesKey)
}

func Contains(db StateAccess, hash common.Hash) bool {
	return keyset.ContainsValue(db, AllEntitiesKey, hash)
}
