package entity

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
)

func Delete(access StateAccess, toDelete common.Hash) (common.Address, error) {

	if !allentities.Contains(access, toDelete) {
		return common.Address{}, fmt.Errorf("entity %s does not exist", toDelete)
	}

	md, err := GetEntityMetaData(access, toDelete)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to get entity meta data: %w", err)
	}

	err = allentities.RemoveEntity(access, toDelete)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to remove entity from all entities: %w", err)
	}

	err = entityexpiration.RemoveFromEntitiesToExpire(access, md.ExpiresAtBlock, toDelete)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to remove entity from entities to expire: %w", err)
	}

	DeletePayload(access, toDelete)
	DeleteEntityMetadata(access, toDelete)

	return md.Owner, nil
}
