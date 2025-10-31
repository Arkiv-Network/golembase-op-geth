package sqlstore

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/logs"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/klauspost/compress/zstd"
)

var decoder, _ = zstd.NewReader(nil)

func WriteLogForBlockSqlite(
	sqlStore *SQLStore,
	db *state.CachingDB,
	hc *core.HeaderChain,
	block *types.Block,
	chainID *big.Int,
	receipts []*types.Receipt,
) (err error) {

	ctx := context.Background()

	writeLog := func() (err error) {

		defer func() {
			if err != nil {
				log.Error("failed to write log for block", "block", block.NumberU64(), "error", err)
			}
		}()

		networkID := chainID.String()

		processingStatus, err := sqlStore.GetProcessingStatus(ctx, networkID)
		if err != nil {
			return fmt.Errorf("failed to get processing status 11: %w", err)
		}

		var haveToResync bool
		switch {
		case processingStatus.LastProcessedBlockNumber == 0 && block.NumberU64() == 1:
			haveToResync = false
		case processingStatus.LastProcessedBlockNumber == 0 && block.NumberU64() != 1:
			haveToResync = true
		case processingStatus.LastProcessedBlockNumber != int64(block.NumberU64()-1):
			haveToResync = true
		case processingStatus.LastProcessedBlockHash != block.ParentHash().Hex():
			haveToResync = true
		default:
			haveToResync = false
		}

		log.Info(
			"processing status",
			"lastProcessedBlockNumber", processingStatus.LastProcessedBlockNumber,
			"lastProcessedBlockHash", processingStatus.LastProcessedBlockHash,
			"block", block.NumberU64(),
			"parentHash", block.ParentHash().Hex(),
			"haveToResync", haveToResync,
		)

		if haveToResync {

			log.Info("resyncing", "block", block.NumberU64(), "parentHash", block.ParentHash().Hex())

			entityIterator := func(
				yield func(*struct {
					Key      common.Hash
					Metadata entity.EntityMetaData
					Payload  []byte
				},
					error,
				) bool,
			) {

				parentHash := hc.GetHeaderByHash(block.ParentHash())
				statedb, err := state.New(parentHash.Root, db)
				if err != nil {
					yield(nil, fmt.Errorf("failed to get statedb: %w", err))
					return
				}

				log.Info("starting entity iteration")

				for entityKey := range allentities.Iterate(statedb) {
					log.Info("iterating over entity", "entityKey", entityKey.Hex())
					emd, err := entity.GetEntityMetaData(statedb, entityKey)
					if err != nil {
						yield(nil, fmt.Errorf("failed to get entity metadata for key %s: %w", entityKey.Hex(), err))
						return
					}
					payload := entity.GetCompressedPayload(statedb, entityKey)

					if !yield(&struct {
						Key      common.Hash
						Metadata entity.EntityMetaData
						Payload  []byte
					}{
						Key:      entityKey,
						Metadata: *emd,
						Payload:  payload,
					}, nil) {
						return
					}
				}
			}

			log.Info("resyncing -1", "block", block.NumberU64(), "parentHash", block.ParentHash().Hex())

			if block.NumberU64() == uint64(1) {

				// for genesis block, we need to iterate over all entities in the database, this is an empty iterator

				log.Info("resyncing on top of genesis block", "block", block.NumberU64(), "parentHash", block.ParentHash().Hex())
				entityIterator = func(
					yield func(*struct {
						Key      common.Hash
						Metadata entity.EntityMetaData
						Payload  []byte
					},
						error,
					) bool,
				) {

				}
			}

			err = sqlStore.SnapSyncToBlock(ctx, chainID.String(), block.NumberU64()-1, block.ParentHash(), entityIterator)
			if err != nil {
				return fmt.Errorf("failed to snap sync to block: %w", err)
			}

		}

		txns := block.Transactions()

		signer := types.LatestSignerForChainID(chainID)

		wal := BlockWal{
			BlockInfo: BlockInfo{
				Number:     block.NumberU64(),
				Hash:       block.Hash(),
				ParentHash: block.ParentHash(),
			},
			Operations: []Operation{},
		}

		for txIx, tx := range txns {
			receipt := receipts[txIx]
			if receipt.Status == types.ReceiptStatusFailed {
				continue
			}

			// quick fix to unblock kaolin
			if len(tx.Data()) == 0 {
				continue
			}

			toAddr := common.Address{}
			if tx.To() != nil {
				toAddr = *tx.To()
			}

			switch {
			case tx.Type() == types.DepositTxType:
				delIx := uint64(0)
				for _, l := range receipt.Logs {
					if len(l.Topics) != 3 {
						continue
					}

					if l.Topics[0] != logs.ArkivEntityExpired {
						continue
					}

					key := l.Topics[1]

					wal.Operations = append(wal.Operations, Operation{
						Delete: &Delete{
							EntityKey:        key,
							TransactionIndex: uint64(txIx),
							OperationIndex:   delIx,
						},
					})
					delIx += 1

				}

			case toAddr == address.ArkivProcessorAddress:

				d, err := decoder.DecodeAll(tx.Data(), nil)
				if err != nil {
					return fmt.Errorf("failed to decode compressed storage transaction: %w", err)
				}

				stx := storagetx.ArkivTransaction{}
				err = rlp.DecodeBytes(d, &stx)
				if err != nil {
					return fmt.Errorf("failed to decode storage transaction: %w", err)
				}

				from, err := types.Sender(signer, tx)
				if err != nil {
					return fmt.Errorf("failed to get sender of create transaction %s: %w", tx.Hash().Hex(), err)
				}

				ops := extractArkivOperations(&stx, txIx, receipt, from)
				wal.Operations = append(wal.Operations, ops...)

			case toAddr == address.GolemBaseStorageProcessorAddress:

				stx := storagetx.StorageTransaction{}
				err := rlp.DecodeBytes(tx.Data(), &stx)
				if err != nil {
					return fmt.Errorf("failed to decode storage transaction: %w", err)
				}

				from, err := types.Sender(signer, tx)
				if err != nil {
					return fmt.Errorf("failed to get sender of create transaction %s: %w", tx.Hash().Hex(), err)
				}

				ops := extractArkivOperations(stx.ConvertToArkiv(), txIx, receipt, from)
				wal.Operations = append(wal.Operations, ops...)

			default:
			}

		}

		err = sqlStore.InsertBlock(
			ctx,
			wal,
			networkID,
		)
		if err != nil {
			return fmt.Errorf("failed to insert block: %w", err)
		}
		return nil
	}

	for {
		err = writeLog()
		if err != nil {
			log.Error("failed to write log", "error", err, "block", block.NumberU64(), "parentHash", block.ParentHash().Hex())
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	return nil
}

var encoder, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))

func extractArkivOperations(
	stx *storagetx.ArkivTransaction,
	txIx int,
	receipt *types.Receipt,
	from common.Address,
) []Operation {
	ops := []Operation{}

	createdLogs := []*types.Log{}
	updatedLogs := []*types.Log{}
	extendedLogs := []*types.Log{}
	changedOwnerLogs := []*types.Log{}

	for _, log := range receipt.Logs {
		if len(log.Topics) < 2 {
			continue
		}

		if log.Topics[0] == logs.ArkivEntityCreated {
			createdLogs = append(createdLogs, log)
		}

		if log.Topics[0] == logs.ArkivEntityUpdated {
			updatedLogs = append(updatedLogs, log)
		}

		if log.Topics[0] == logs.ArkivEntityBTLExtended {
			extendedLogs = append(extendedLogs, log)
		}

		if log.Topics[0] == logs.ArkivEntityOwnerChanged {
			changedOwnerLogs = append(changedOwnerLogs, log)
		}
	}

	for opIx, create := range stx.Create {

		l := createdLogs[opIx]
		key := l.Topics[1]
		expiresAtBlockU256 := uint256.NewInt(0).SetBytes(l.Data[:32])
		expiresAtBlock := expiresAtBlockU256.Uint64()

		cr := Create{
			EntityKey:          key,
			ExpiresAtBlock:     expiresAtBlock,
			Payload:            encoder.EncodeAll(create.Payload, nil),
			ContentType:        create.ContentType,
			StringAnnotations:  create.StringAnnotations,
			NumericAnnotations: create.NumericAnnotations,
			Owner:              from,
			TransactionIndex:   uint64(txIx),
			OperationIndex:     uint64(opIx),
		}

		ops = append(ops, Operation{
			Create: &cr,
		})

	}

	for opIx, del := range stx.Delete {
		ops = append(ops, Operation{
			Delete: &Delete{
				EntityKey:        del,
				TransactionIndex: uint64(txIx),
				OperationIndex:   uint64(opIx),
			},
		})
	}

	for opIx, update := range stx.Update {

		l := updatedLogs[opIx]
		key := l.Topics[1]
		expiresAtBlockU256 := uint256.NewInt(0).SetBytes(l.Data[32:64])
		expiresAtBlock := expiresAtBlockU256.Uint64()

		ur := Update{
			EntityKey:          key,
			ExpiresAtBlock:     expiresAtBlock,
			Payload:            encoder.EncodeAll(update.Payload, nil),
			ContentType:        update.ContentType,
			StringAnnotations:  update.StringAnnotations,
			NumericAnnotations: update.NumericAnnotations,
			TransactionIndex:   uint64(txIx),
			OperationIndex:     uint64(opIx),
		}

		ops = append(ops, Operation{
			Update: &ur,
		})
	}

	for opIx, extend := range stx.Extend {

		l := extendedLogs[opIx]

		oldExpiresAtU256 := uint256.NewInt(0).SetBytes(l.Data[:32])
		oldExpiresAt := oldExpiresAtU256.Uint64()

		newExpiresAtU256 := uint256.NewInt(0).SetBytes(l.Data[32:64])
		newExpiresAt := newExpiresAtU256.Uint64()

		ex := ExtendBTL{
			EntityKey:        extend.EntityKey,
			OldExpiresAt:     oldExpiresAt,
			NewExpiresAt:     newExpiresAt,
			TransactionIndex: uint64(txIx),
			OperationIndex:   uint64(opIx),
		}

		ops = append(ops, Operation{
			Extend: &ex,
		})
	}

	for opIx, changeOwner := range stx.ChangeOwner {

		log := changedOwnerLogs[opIx]

		owner := common.BytesToAddress(log.Topics[3].Bytes())

		co := ChangeOwner{
			EntityKey:        changeOwner.EntityKey,
			Owner:            owner,
			TransactionIndex: uint64(txIx),
			OperationIndex:   uint64(opIx),
		}

		ops = append(ops, Operation{
			ChangeOwner: &co,
		})
	}

	return ops
}
