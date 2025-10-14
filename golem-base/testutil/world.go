package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
)

// World is the test world - it holds all the state that is shared between steps
type World struct {
	GethInstance           *GethInstance
	FundedAccount          *FundedAccount
	SecondFundedAccount    *FundedAccount
	LastReceipt            *types.Receipt
	SearchResult           []golemtype.SearchResult
	CreatedEntityKey       common.Hash
	SecondCreatedEntityKey common.Hash
	LastError              error
	LastTrace              json.RawMessage

	// Storage transaction validation fields
	CurrentStorageTransaction *storagetx.StorageTransaction
	ValidationError           error

	tempDir string
}

func NewWorld(ctx context.Context, gethPath string) (*World, error) {
	td, err := os.MkdirTemp("", "golem-base")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	geth, err := startGethInstance(ctx, gethPath, td)
	if err != nil {
		return nil, fmt.Errorf("failed to start geth instance: %w", err)
	}

	var acc *FundedAccount
	for i := range 10 {
		acc, err = geth.createAccountAndTransferFunds(ctx, EthToWei(100))
		if err == nil {
			break
		} else {
			if i < 9 {
				continue
			} else {
				return nil, fmt.Errorf("failed to create account and transfer funds: %w", err)
			}
		}
	}

	var acc2 *FundedAccount
	for i := range 10 {
		acc2, err = geth.createAccountAndTransferFunds(ctx, EthToWei(100))
		if err == nil {
			break
		} else {
			if i < 9 {
				continue
			} else {
				return nil, fmt.Errorf("failed to create account and transfer funds: %w", err)
			}
		}
	}

	return &World{
		GethInstance:        geth,
		FundedAccount:       acc,
		SecondFundedAccount: acc2,
		tempDir:             td,
	}, nil

}

func (w *World) Shutdown() {
	w.GethInstance.shutdown()
	os.RemoveAll(w.tempDir)
}

func (w *World) AddLogsToTestError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%w\n\nGeth Logs:\n%s", err, w.GethInstance.output.String())
}
