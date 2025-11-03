package query

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

func Query() *cli.Command {
	cfg := struct {
		nodeURL string
		NoData  bool
	}{}
	return &cli.Command{
		Name:  "query",
		Usage: "query entity",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "node-url",
				Usage:       "The URL of the node to connect to",
				Value:       "http://localhost:8545",
				EnvVars:     []string{"NODE_URL"},
				Destination: &cfg.nodeURL,
			},
			&cli.BoolFlag{
				Name:        "no-data",
				Usage:       "Do not print the stored value",
				Destination: &cfg.NoData,
				EnvVars:     []string{"NO_DATA"},
			},
		},
		Action: func(c *cli.Context) error {

			ctx, stop := signal.NotifyContext(c.Context, os.Interrupt)
			defer stop()

			query := c.Args().First()
			if query == "" {
				return fmt.Errorf("query string is required")
			}
			// Connect to the geth node
			rpcClient, err := rpc.Dial(cfg.nodeURL)
			if err != nil {
				return fmt.Errorf("failed to connect to node: %w", err)
			}
			defer rpcClient.Close()

			res := []golemtype.SearchResult{}

			err = rpcClient.CallContext(
				ctx,
				&res,
				"golembase_queryEntities",
				query,
			)
			if err != nil {
				return fmt.Errorf("failed to get entities by numeric annotation: %w", err)
			}

			for _, r := range res {
				fmt.Println(r.Key)
				if !cfg.NoData {
					fmt.Println("  payload:", string(r.Value))
				}
			}

			return nil
		},
	}
}
