package integrity

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

func Integrity() *cli.Command {
	cfg := struct {
		nodeURL string
	}{}
	return &cli.Command{
		Name:  "integrity",
		Usage: "check integrity of all entities by trying to simulate the deletion of each entity",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "node-url",
				Usage:       "The URL of the node to connect to",
				Value:       "http://localhost:8545",
				EnvVars:     []string{"NODE_URL"},
				Destination: &cfg.nodeURL,
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

			db := &RPCStateAccess{rpcClient, ctx}

			for entityHash := range allentities.Iterate(db) {
				fmt.Println("checking", entityHash)
				_, err = entity.Delete(db, entityHash)
				if err != nil {
					fmt.Println("error deleting", err)
				}
			}

			return nil
		},
	}
}

type RPCStateAccess struct {
	rpcClient *rpc.Client
	ctx       context.Context
}

func (s *RPCStateAccess) GetState(a common.Address, slot common.Hash) common.Hash {

	var res common.Hash
	err := s.rpcClient.CallContext(s.ctx, &res, "eth_getStorageAt", a, slot, "latest")
	if err != nil {
		panic(err)
	}
	return res
}

func (s *RPCStateAccess) SetState(common.Address, common.Hash, common.Hash) common.Hash {
	return common.Hash{}
}
