package update

import (
	"fmt"
	"math/big"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/cmd/golembase/account/pkg/useraccount"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/urfave/cli/v2"
)

func Update() *cli.Command {
	cfg := struct {
		nodeURL string
		data    string
		key     string
		btl     uint64
	}{}
	return &cli.Command{
		Name:  "update",
		Usage: "Update an existing entity",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "node-url",
				Usage:       "The URL of the node to connect to",
				Value:       "http://localhost:8545",
				EnvVars:     []string{"NODE_URL"},
				Destination: &cfg.nodeURL,
			},
			&cli.StringFlag{
				Name:        "key",
				Usage:       "key of the entity to update",
				Required:    true,
				EnvVars:     []string{"ENTITY_KEY"},
				Destination: &cfg.key,
			},
			&cli.StringFlag{
				Name:        "data",
				Usage:       "new data for the update operation",
				Value:       "this is updated data",
				EnvVars:     []string{"ENTITY_DATA"},
				Destination: &cfg.data,
			},
			&cli.Uint64Flag{
				Name:        "btl",
				Usage:       "new btl for the update operation",
				Value:       100,
				EnvVars:     []string{"ENTITY_BTL"},
				Destination: &cfg.btl,
			},
		},
		Action: func(c *cli.Context) error {
			ctx, cancel := signal.NotifyContext(c.Context, os.Interrupt)
			defer cancel()

			userAccount, err := useraccount.Load()
			if err != nil {
				return fmt.Errorf("failed to load user account: %w", err)
			}

			// Connect to the geth node
			client, err := ethclient.DialContext(ctx, cfg.nodeURL)
			if err != nil {
				return fmt.Errorf("failed to connect to node: %w", err)
			}
			defer client.Close()

			// Get the chain ID
			chainID, err := client.ChainID(ctx)
			if err != nil {
				return fmt.Errorf("failed to get chain ID: %w", err)
			}

			// Get the nonce for the sender account
			nonce, err := client.PendingNonceAt(ctx, userAccount.Address)
			if err != nil {
				return fmt.Errorf("failed to get nonce: %w", err)
			}

			// Create the storage transaction
			storageTx := &storagetx.StorageTransaction{
				Update: []storagetx.Update{
					{
						EntityKey: common.HexToHash(c.String("key")),
						BTL:       cfg.btl,
						Payload:   []byte(c.String("data")),
						StringAnnotations: []entity.StringAnnotation{
							{
								Key:   "foo",
								Value: "bar",
							},
						},
					},
				},
			}

			// Encode the storage transaction
			txData, err := rlp.EncodeToBytes(storageTx)
			if err != nil {
				return fmt.Errorf("failed to encode storage tx: %w", err)
			}

			// Create the GolemBaseUpdateStorageTx
			tx := &types.DynamicFeeTx{
				ChainID:   chainID,
				Nonce:     nonce,
				Gas:       1_000_000,
				Data:      txData,
				To:        &address.GolemBaseStorageProcessorAddress,
				GasTipCap: big.NewInt(1e9), // 1 Gwei
				GasFeeCap: big.NewInt(5e9), // 5 Gwei
			}

			// Use the London signer since we're using a dynamic fee transaction
			signer := types.LatestSignerForChainID(chainID)

			// Create and sign the transaction
			signedTx, err := types.SignNewTx(userAccount.PrivateKey, signer, tx)
			if err != nil {
				return fmt.Errorf("failed to sign transaction: %w", err)
			}

			txHash := signedTx.Hash()

			err = client.SendTransaction(ctx, signedTx)
			if err != nil {
				return fmt.Errorf("failed to send tx: %w", err)
			}

			receipt, err := bind.WaitMinedHash(ctx, client, txHash)
			if err != nil {
				return fmt.Errorf("failed to wait for tx: %w", err)
			}

			if receipt.Status != types.ReceiptStatusSuccessful {
				return fmt.Errorf("tx failed")
			}

			for _, log := range receipt.Logs {
				if log.Topics[0] == storagetx.GolemBaseStorageEntityUpdated {
					fmt.Println("Entity updated", "key", log.Topics[1])
				}
			}

			return nil
		},
	}
}
