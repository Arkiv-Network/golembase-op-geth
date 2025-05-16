package testutil

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/rlp"
)

func (w *World) UpdateEntity(
	ctx context.Context,
	key common.Hash,
	btl uint64,
	payload []byte,
	stringAnnotations []entity.StringAnnotation,
	numericAnnotations []entity.NumericAnnotation,
) (*types.Receipt, error) {

	receipt, err := w.updateEntity(
		ctx,
		key,
		w.FundedAccount,
		btl,
		payload,
		stringAnnotations,
		numericAnnotations,
	)
	if err != nil {
		return nil, err
	}

	if receipt.Status == types.ReceiptStatusFailed {
		return nil, fmt.Errorf("transaction failed")
	}

	return receipt, nil

}

func (w *World) UpdateEntityBySecondAccount(
	ctx context.Context,
	key common.Hash,
	btl uint64,
	payload []byte,
	stringAnnotations []entity.StringAnnotation,
	numericAnnotations []entity.NumericAnnotation,
) (*types.Receipt, error) {

	receipt, err := w.updateEntity(
		ctx,
		key,
		w.SecondFundedAccount,
		btl,
		payload,
		stringAnnotations,
		numericAnnotations,
	)
	if err != nil {
		return nil, err
	}

	return receipt, nil

}

func (w *World) updateEntity(
	ctx context.Context,
	key common.Hash,
	account *FundedAccount,
	btl uint64,
	payload []byte,
	stringAnnotations []entity.StringAnnotation,
	numericAnnotations []entity.NumericAnnotation,
) (*types.Receipt, error) {

	client := w.GethInstance.ETHClient

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Get the current nonce for the sender address
	nonce, err := client.PendingNonceAt(ctx, account.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Create a StorageTransaction with a single Create operation
	storageTx := &storagetx.StorageTransaction{
		Update: []storagetx.Update{
			{
				EntityKey:          key,
				BTL:                btl,
				Payload:            payload,
				StringAnnotations:  stringAnnotations,
				NumericAnnotations: numericAnnotations,
			},
		},
	}

	// RLP encode the storage transaction
	rlpData, err := rlp.EncodeToBytes(storageTx)
	if err != nil {
		return nil, fmt.Errorf("failed to encode storage transaction: %w", err)
	}

	// Create UpdateStorageTx instance with the RLP encoded data
	txdata := &types.DynamicFeeTx{
		ChainID:    chainID,
		Nonce:      nonce,
		GasTipCap:  big.NewInt(1e9), // 1 Gwei
		GasFeeCap:  big.NewInt(5e9), // 5 Gwei
		Gas:        100_000,
		To:         &address.GolemBaseStorageProcessorAddress,
		Value:      big.NewInt(0), // No ETH transfer needed
		Data:       rlpData,
		AccessList: types.AccessList{},
	}

	// Use the London signer since we're using a dynamic fee transaction
	signer := types.LatestSignerForChainID(chainID)

	// Create and sign the transaction
	signedTx, err := types.SignNewTx(account.PrivateKey, signer, txdata)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for transaction: %w", err)
	}

	w.LastReceipt = receipt

	return receipt, nil

}
