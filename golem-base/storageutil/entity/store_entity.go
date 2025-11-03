package entity

import (
	"fmt"
	"regexp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
	"github.com/klauspost/compress/zstd"
)

// This regex should not allow $ or 0x as the first characters, since we use that for
// special meta-annotations like $owner and for hashes and addresses.
const AnnotationIdentRegex string = `[\p{L}_][\p{L}\p{N}_]*`

var AnnotationIdentRegexCompiled *regexp.Regexp = regexp.MustCompile(fmt.Sprintf("^%s$", AnnotationIdentRegex))

type StateAccess = storageutil.StateAccess

var encoder, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))

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
