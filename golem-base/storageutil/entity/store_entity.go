package entity

import (
	"fmt"
	"regexp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entitiesofowner"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
)

// This regex should not allow $ as the first character, since we use that for
// special meta-annotations like $owner.
const AnnotationIdentRegex string = `[\p{L}_][\p{L}\p{N}_]*`

var AnnotationIdentRegexCompiled *regexp.Regexp = regexp.MustCompile(fmt.Sprintf("^%s$", AnnotationIdentRegex))

type StateAccess = storageutil.StateAccess

func Store(
	access StateAccess,
	key common.Hash,
	sender common.Address,
	emd EntityMetaData,
	payload []byte,
) error {

	err := allentities.AddEntity(access, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to all entities: %w", err)
	}

	err = entitiesofowner.AddEntity(access, sender, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to owner entities: %w", err)
	}

	err = StoreEntityMetaData(access, key, emd)
	if err != nil {
		return fmt.Errorf("failed to store entity meta data: %w", err)
	}

	err = entityexpiration.AddToEntitiesToExpireAtBlock(access, emd.ExpiresAtBlock, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to entities to expire: %w", err)
	}

	StorePayload(access, key, payload)

	return nil
}
