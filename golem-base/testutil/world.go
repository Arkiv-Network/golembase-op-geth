package testutil

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
)

// World is the test world - it holds all the state that is shared between steps
type World struct {
	GethInstance        *GethInstance
	FundedAccount       *FundedAccount
	SecondFundedAccount *FundedAccount
	LastReceipt         *types.Receipt
	SearchResult        []golemtype.SearchResult
	CreatedEntityKey    common.Hash
	LastError           error
}

func NewWorld(ctx context.Context, gethPath string) (*World, error) {
	geth, err := startGethInstance(ctx, gethPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start geth instance: %w", err)
	}

	acc, err := geth.createAccountAndTransferFunds(ctx, EthToWei(100))
	if err != nil {
		return nil, fmt.Errorf("failed to create account and transfer funds: %w", err)
	}

	acc2, err := geth.createAccountAndTransferFunds(ctx, EthToWei(100))
	if err != nil {
		return nil, fmt.Errorf("failed to create account and transfer funds: %w", err)
	}

	return &World{
		GethInstance:        geth,
		FundedAccount:       acc,
		SecondFundedAccount: acc2,
	}, nil

}

func (w *World) Shutdown() {
	w.GethInstance.shutdown()
}

func (w *World) AddLogsToTestError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%w\n\nGeth Logs:\n%s", err, w.GethInstance.output.String())
}
