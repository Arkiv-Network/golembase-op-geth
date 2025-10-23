package housekeepingtx

import (
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/golem-base/address"
	arkivlogs "github.com/ethereum/go-ethereum/golem-base/logs"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
)

func addressToHash(a common.Address) common.Hash {
	h := common.Hash{}
	copy(h[12:], a[:])
	return h
}

func ExecuteTransaction(blockNumber uint64, txHash common.Hash, db vm.StateDB) (_ []*types.Log, err error) {

	// create the golem base storage processor address if it doesn't exist
	// this is needed to be able to use the state access interface
	if !db.Exist(address.ArkivProcessorAddress) {
		db.CreateAccount(address.ArkivProcessorAddress)
		db.CreateContract(address.ArkivProcessorAddress)
		db.SetNonce(address.ArkivProcessorAddress, 1, tracing.NonceChangeNewContract)
	}

	logs := []*types.Log{}

	st := storageaccounting.NewSlotUsageCounter(db)

	defer func() {
		if err == nil {
			st.UpdateUsedSlotsForGolemBase()
		}
	}()

	deleteEntity := func(toDelete common.Hash) error {

		owner, err := entity.Delete(st, toDelete)
		if err != nil {
			return fmt.Errorf("failed to delete entity: %w", err)
		}

		// create the log for the created entity
		logs = append(
			logs,
			&types.Log{
				Address:     address.GolemBaseStorageProcessorAddress, // Set the appropriate address if needed
				Topics:      []common.Hash{storagetx.GolemBaseStorageEntityDeleted, toDelete},
				Data:        []byte{},
				BlockNumber: blockNumber,
			},
			&types.Log{
				Address: common.Address(address.ArkivProcessorAddress),
				Topics: []common.Hash{
					arkivlogs.ArkivEntityExpired,
					toDelete,
					addressToHash(owner),
				},
				Data:        []byte{},
				BlockNumber: blockNumber,
			},
		)

		return nil
	}

	toDelete := slices.Collect(entityexpiration.IteratorOfEntitiesToExpireAtBlock(st, blockNumber))

	for _, key := range toDelete {
		err := deleteEntity(key)
		if err != nil {
			return nil, fmt.Errorf("failed to delete entity %s: %w", key.Hex(), err)
		}
	}

	return logs, nil
}
