package create

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/cmd/golembase/account/pkg/useraccount"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/golem-base/address"
	arkivlogs "github.com/ethereum/go-ethereum/golem-base/logs"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/urfave/cli/v2"
)

// To supply string annotations, provide separate --string
// flags for each annotation. After each flag, pass the
// pair as key:value (separated by a colon).
// Example:
// --string hello:world --string foo:bar
// to provide two annotations, hello:world and foo:bar.

func ParseStringAnnotations(input []string) ([]entity.StringAnnotation, error) {
	var annotations []entity.StringAnnotation

	for _, pair := range input {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid annotation pair: %q", pair)
		}
		annotations = append(annotations, entity.StringAnnotation{
			Key:   strings.TrimSpace(kv[0]),
			Value: strings.TrimSpace(kv[1]),
		})
	}

	return annotations, nil
}

// To supply numeric annotations, provide separate --num
// flags for each annotation. After each flag, pass the
// pair as key:value (separated by a colon).
// Example:
// --num favorite:100 --num count:10
// to provide two annotations, favorite:100 and count:10.
func ParseNumericAnnotations(input []string) ([]entity.NumericAnnotation, error) {
	var annotations []entity.NumericAnnotation

	for _, pair := range input {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		valStr := strings.TrimSpace(kv[1])

		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value for key %q: %v", key, err)
		}

		annotations = append(annotations, entity.NumericAnnotation{
			Key:   key,
			Value: val,
		})
	}

	return annotations, nil
}

func Create() *cli.Command {

	cfg := struct {
		nodeURL string
		data    string
		btl     uint64
	}{}
	return &cli.Command{
		Name:  "create",
		Usage: "Create a new entity",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "node-url",
				Usage:       "The URL of the node to connect to",
				Value:       "http://localhost:8545",
				EnvVars:     []string{"NODE_URL"},
				Destination: &cfg.nodeURL,
			},
			&cli.StringFlag{
				Name:        "data",
				Usage:       "data for the create operation",
				Value:       "this is a test",
				EnvVars:     []string{"ENTITY_DATA"},
				Destination: &cfg.data,
			},
			&cli.Uint64Flag{
				Name:        "btl",
				Usage:       "btl for the create operation",
				Value:       100,
				EnvVars:     []string{"ENTITY_BTL"},
				Destination: &cfg.btl,
			},
			&cli.StringSliceFlag{
				Name:    "string",
				Aliases: []string{"s"},
				Usage:   "Key/Value for string annotation. Specify as foo:bar. Pass multiple instances of --string as needed",
			},
			&cli.StringSliceFlag{
				Name:    "num",
				Aliases: []string{"n"},
				Usage:   "Key/Value for numeric annotation. Specify as favorite:100. Pass multiple instances of --num as needed",
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

			strs, err := ParseStringAnnotations(c.StringSlice("string"))
			if err != nil {
				return fmt.Errorf("failed to parse string annotations: %w", err)
			}

			nums, err := ParseNumericAnnotations(c.StringSlice("num"))
			if err != nil {
				return fmt.Errorf("failed to parse numeric annotations: %w", err)
			}

			// Create the storage transaction
			storageTx := &storagetx.StorageTransaction{
				Create: []storagetx.Create{
					{
						BTL:     cfg.btl,
						Payload: []byte(c.String("data")),

						StringAnnotations:  strs,
						NumericAnnotations: nums,
					},
				},
			}

			// Encode the storage transaction
			txData, err := rlp.EncodeToBytes(storageTx)
			if err != nil {
				return fmt.Errorf("failed to encode storage tx: %w", err)
			}

			// Dynamically determine gas, gas tip cap, and gas fee cap
			msg := ethereum.CallMsg{
				From:     userAccount.Address,
				To:       &address.GolemBaseStorageProcessorAddress,
				Gas:      0, // let EstimateGas determine
				GasPrice: nil,
				Value:    nil,
				Data:     txData,
			}

			gasLimit, err := client.EstimateGas(ctx, msg)
			if err != nil {
				return fmt.Errorf("failed to estimate gas: %w", err)
			}

			gasTipCap, err := client.SuggestGasTipCap(ctx)
			if err != nil {
				return fmt.Errorf("failed to suggest gas tip cap: %w", err)
			}

			gasFeeCap, err := client.SuggestGasPrice(ctx)
			if err != nil {
				return fmt.Errorf("failed to suggest gas fee cap: %w", err)
			}

			// Create the GolemBaseUpdateStorageTx
			tx := &types.DynamicFeeTx{
				ChainID:   chainID,
				Nonce:     nonce,
				Gas:       gasLimit,
				Data:      txData,
				To:        &address.GolemBaseStorageProcessorAddress,
				GasTipCap: gasTipCap,
				GasFeeCap: gasFeeCap,
			}

			// Use the London signer since we're using a dynamic fee transaction
			signer := types.LatestSignerForChainID(chainID)

			// return nil, fmt.Errorf("signer: %#v", signer)

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
				if log.Topics[0] == arkivlogs.ArkivEntityCreated {
					fmt.Println("Entity created", "key", log.Topics[1])
				}
			}

			return nil
		},
	}
}

func pointerOf[T any](v T) *T {
	return &v
}
