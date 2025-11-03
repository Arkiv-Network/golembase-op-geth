package allentities

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"
	"github.com/urfave/cli/v2"
)

func AllEntities() *cli.Command {
	cfg := struct {
		nodeURL string
		block   uint64
	}{}
	return &cli.Command{
		Name:  "all-entities",
		Usage: "List state for all entities",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "node-url",
				Usage:       "The URL of the node to connect to",
				Value:       "http://localhost:8545",
				EnvVars:     []string{"NODE_URL"},
				Destination: &cfg.nodeURL,
			},
			&cli.Uint64Flag{
				Name:        "block",
				Usage:       "The block number to list state for",
				Value:       0,
				Destination: &cfg.block,
			},
		},
		Action: func(c *cli.Context) error {

			ctx, stop := signal.NotifyContext(c.Context, os.Interrupt)
			defer stop()

			rpcClient, err := rpc.Dial(cfg.nodeURL)
			if err != nil {
				return fmt.Errorf("failed to connect to node: %w", err)
			}
			defer rpcClient.Close()

			var res common.Hash

			firstSlot := allentities.AllEntitiesKey

			var block any = "latest"
			if cfg.block != 0 {
				block = hexutil.Uint64(cfg.block)
			}

			err = rpcClient.CallContext(ctx, &res, "eth_getStorageAt", address.ArkivProcessorAddress, firstSlot, block)
			if err != nil {
				return fmt.Errorf("failed to get storage at: %w", err)
			}

			fmt.Println(firstSlot, res)

			numberOfEntities := new(uint256.Int).SetBytes(res[:])

			curentSlot := new(uint256.Int).SetBytes(firstSlot[:])

			fmt.Println(numberOfEntities.Uint64())
			for i := uint64(0); i < numberOfEntities.Uint64(); i++ {

				curentSlot.Add(curentSlot, uint256.NewInt(1))

				currentSlotHash := common.Hash(curentSlot.Bytes32())

				err = rpcClient.CallContext(ctx, &res, "eth_getStorageAt", address.ArkivProcessorAddress, currentSlotHash, block)
				if err != nil {
					return fmt.Errorf("failed to get storage at: %w", err)
				}
				fmt.Println(currentSlotHash, res)

				mappingSlot := crypto.Keccak256Hash(keyset.MapKeyPrefix, allentities.AllEntitiesKey[:], res[:])

				err = rpcClient.CallContext(ctx, &res, "eth_getStorageAt", address.ArkivProcessorAddress, mappingSlot, block)
				if err != nil {
					return fmt.Errorf("failed to get storage at: %w", err)
				}
				fmt.Println(mappingSlot, res)
			}

			return nil
		},
	}
}
