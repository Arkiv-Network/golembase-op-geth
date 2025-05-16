package wal

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

type BlockInfo struct {
	Number     uint64      `json:"number,string"`
	Hash       common.Hash `json:"hash"`
	ParentHash common.Hash `json:"parentHash"`
}

type Operation struct {
	Create *Create      `json:"create,omitempty"`
	Update *Update      `json:"update,omitempty"`
	Delete *common.Hash `json:"delete,omitempty"`
	Extend *ExtendBTL   `json:"extend,omitempty"`
}

type Create struct {
	EntityKey          common.Hash                `json:"entityKey"`
	ExpiresAtBlock     uint64                     `json:"expiresAtBlock"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
	Owner              common.Address             `json:"owner"`
}

type Update struct {
	EntityKey          common.Hash                `json:"entityKey"`
	ExpiresAtBlock     uint64                     `json:"expiresAtBlock"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
}

type ExtendBTL struct {
	EntityKey    common.Hash `json:"entityKey"`
	OldExpiresAt uint64      `json:"oldExpiresAt"`
	NewExpiresAt uint64      `json:"newExpiresAt"`
}

func BlockNumberToFilename(blockNumber uint64) string {
	return fmt.Sprintf("block-%020d.json", blockNumber)
}

var ErrInvalidFilename = errors.New("invalid filename")

var re = regexp.MustCompile(`^block-(\d+)\.json$`)

func PathToBlockNumber(path string) (uint64, error) {

	fn := filepath.Base(path)

	matches := re.FindStringSubmatch(fn)
	if len(matches) != 2 {
		return 0, ErrInvalidFilename
	}

	return strconv.ParseUint(matches[1], 10, 64)
}

func WriteLogForBlock(dir string, block *types.Block, chainID *big.Int, receipts []*types.Receipt) (err error) {

	defer func() {
		if err != nil {
			log.Error("failed to write log for block", "block", block.NumberU64(), "error", err)
		}
	}()

	tempFilename := BlockNumberToFilename(block.NumberU64()) + ".temp"
	finalFilename := BlockNumberToFilename(block.NumberU64())

	tf, err := os.OpenFile(filepath.Join(dir, tempFilename), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer func() {
		tf.Close()
		os.Remove(filepath.Join(dir, tempFilename))
	}()

	enc := json.NewEncoder(tf)

	enc.Encode(BlockInfo{
		Number:     block.NumberU64(),
		Hash:       block.Hash(),
		ParentHash: block.ParentHash(),
	})

	txns := block.Transactions()

	signer := types.LatestSignerForChainID(chainID)

	for i, tx := range txns {
		receipt := receipts[i]
		if receipt.Status == types.ReceiptStatusFailed {
			continue
		}

		toAddr := common.Address{}
		if tx.To() != nil {
			toAddr = *tx.To()
		}

		switch {
		case tx.Type() == types.DepositTxType:
			for _, l := range receipt.Logs {
				if len(l.Topics) != 2 {
					continue
				}

				if l.Topics[0] != storagetx.GolemBaseStorageEntityDeleted {
					continue
				}

				key := l.Topics[1]

				err := enc.Encode(Operation{
					Delete: &key,
				})
				if err != nil {
					return fmt.Errorf("failed to encode delete operation: %w", err)
				}

			}
			// create
		case toAddr == address.GolemBaseStorageProcessorAddress:

			stx := storagetx.StorageTransaction{}
			err := rlp.DecodeBytes(tx.Data(), &stx)
			if err != nil {
				return fmt.Errorf("failed to decode storage transaction: %w", err)
			}

			createdLogs := []*types.Log{}
			updatedLogs := []*types.Log{}
			extendedLogs := []*types.Log{}

			for _, log := range receipt.Logs {
				if len(log.Topics) < 2 {
					continue
				}

				if log.Topics[0] == storagetx.GolemBaseStorageEntityCreated {
					createdLogs = append(createdLogs, log)
				}

				if log.Topics[0] == storagetx.GolemBaseStorageEntityUpdated {
					updatedLogs = append(updatedLogs, log)
				}

				if log.Topics[0] == storagetx.GolemBaseStorageEntityBTLExtended {
					extendedLogs = append(extendedLogs, log)
				}

			}

			for i, create := range stx.Create {

				l := createdLogs[i]
				key := l.Topics[1]
				expiresAtBlockU256 := uint256.NewInt(0).SetBytes(l.Data)
				expiresAtBlock := expiresAtBlockU256.Uint64()

				from, err := types.Sender(signer, tx)
				if err != nil {
					return fmt.Errorf("failed to get sender of create transaction %s: %w", tx.Hash().Hex(), err)
				}

				cr := Create{
					EntityKey:          key,
					ExpiresAtBlock:     expiresAtBlock,
					Payload:            create.Payload,
					StringAnnotations:  create.StringAnnotations,
					NumericAnnotations: create.NumericAnnotations,
					Owner:              from,
				}

				err = enc.Encode(Operation{
					Create: &cr,
				})
				if err != nil {
					return fmt.Errorf("failed to encode create operation: %w", err)
				}

			}

			for _, del := range stx.Delete {
				err := enc.Encode(Operation{
					Delete: &del,
				})
				if err != nil {
					return fmt.Errorf("failed to encode delete operation: %w", err)
				}
			}

			for i, update := range stx.Update {

				log := updatedLogs[i]
				key := log.Topics[1]
				expiresAtBlockU256 := uint256.NewInt(0).SetBytes(log.Data)
				expiresAtBlock := expiresAtBlockU256.Uint64()

				ur := Update{
					EntityKey:          key,
					ExpiresAtBlock:     expiresAtBlock,
					Payload:            update.Payload,
					StringAnnotations:  update.StringAnnotations,
					NumericAnnotations: update.NumericAnnotations,
				}

				err := enc.Encode(Operation{
					Update: &ur,
				})
				if err != nil {
					return fmt.Errorf("failed to encode update operation: %w", err)
				}
			}

			for i, extend := range stx.Extend {

				log := extendedLogs[i]

				oldExpiresAtU256 := uint256.NewInt(0).SetBytes(log.Data[:32])
				oldExpiresAt := oldExpiresAtU256.Uint64()

				newExpiresAtU256 := uint256.NewInt(0).SetBytes(log.Data[32:])
				newExpiresAt := newExpiresAtU256.Uint64()

				ex := ExtendBTL{
					EntityKey:    extend.EntityKey,
					OldExpiresAt: oldExpiresAt,
					NewExpiresAt: newExpiresAt,
				}

				err := enc.Encode(Operation{
					Extend: &ex,
				})
				if err != nil {
					return fmt.Errorf("failed to encode extend operation: %w", err)
				}
			}

		default:
		}

	}

	err = tf.Close()
	if err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	err = os.Rename(filepath.Join(dir, tempFilename), filepath.Join(dir, finalFilename))
	if err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
