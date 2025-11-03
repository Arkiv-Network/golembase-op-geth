package testutil

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (w *World) SendTxWithData(
	ctx context.Context,
	value *big.Int,
	recipient common.Address,
	data []byte,
) (*types.Receipt, error) {

	client := w.GethInstance.ETHClient

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Get the current nonce for the sender address
	nonce, err := client.PendingNonceAt(ctx, w.FundedAccount.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Create UpdateStorageTx instance with the RLP encoded data
	txdata := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: big.NewInt(1e9), // 1 Gwei
		GasFeeCap: big.NewInt(5e9), // 5 Gwei
		Gas:       2_800_000,
		To:        &recipient,
		Value:     value,
		Data:      data,
	}

	// Use the London signer since we're using a dynamic fee transaction
	signer := types.LatestSignerForChainID(chainID)

	// Create and sign the transaction
	signedTx, err := types.SignNewTx(w.FundedAccount.PrivateKey, signer, txdata)
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

func (w *World) SendTxFromSecondAccountWithData(
	ctx context.Context,
	value *big.Int,
	recipient common.Address,
	data []byte,
) (*types.Receipt, error) {

	client := w.GethInstance.ETHClient

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Get the current nonce for the sender address
	nonce, err := client.PendingNonceAt(ctx, w.SecondFundedAccount.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Create UpdateStorageTx instance with the RLP encoded data
	txdata := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: big.NewInt(1e9), // 1 Gwei
		GasFeeCap: big.NewInt(5e9), // 5 Gwei
		Gas:       2_800_000,
		To:        &recipient,
		Value:     value,
		Data:      data,
	}

	// Use the London signer since we're using a dynamic fee transaction
	signer := types.LatestSignerForChainID(chainID)

	// Create and sign the transaction
	signedTx, err := types.SignNewTx(w.SecondFundedAccount.PrivateKey, signer, txdata)
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
