package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/signal"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/golem-base/etl/mongodb/mongogolem"
	"github.com/ethereum/go-ethereum/golem-base/wal"
	"github.com/urfave/cli/v2"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := struct {
		mongoURI    string
		dbName      string
		walDir      string
		rpcEndpoint string
	}{}

	app := &cli.App{
		Name: "mongodb-etl",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "mongo-uri",
				Usage:       "MongoDB connection URI",
				EnvVars:     []string{"MONGO_URI"},
				Destination: &cfg.mongoURI,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "db-name",
				Usage:       "MongoDB database name",
				EnvVars:     []string{"DB_NAME"},
				Destination: &cfg.dbName,
				Required:    true,
			},
			&cli.PathFlag{
				Name:        "wal",
				Usage:       "wal dir",
				EnvVars:     []string{"WAL_DIR"},
				Required:    true,
				Destination: &cfg.walDir,
			},
			&cli.StringFlag{
				Name:        "rpc-endpoint",
				Usage:       "RPC Endpoint for op-geth",
				EnvVars:     []string{"RPC_ENDPOINT"},
				Required:    true,
				Destination: &cfg.rpcEndpoint,
			},
		},
		Action: func(c *cli.Context) error {
			ctx, cancel := signal.NotifyContext(c.Context, os.Interrupt)
			defer cancel()

			// Connect to MongoDB
			clientOptions := options.Client().ApplyURI(cfg.mongoURI)
			client, err := mongo.Connect(ctx, clientOptions)
			if err != nil {
				return fmt.Errorf("failed to connect to MongoDB: %w", err)
			}
			defer func() {
				if err := client.Disconnect(ctx); err != nil {
					log.Error("failed to disconnect from MongoDB", "error", err)
				}
			}()

			// Ping the MongoDB server to ensure connection is established
			if err := client.Ping(ctx, nil); err != nil {
				return fmt.Errorf("failed to ping MongoDB: %w", err)
			}
			log.Info("Connected to MongoDB")

			// Get database and create MongoDB driver
			db := client.Database(cfg.dbName)
			mongoDriver := mongogolem.New(db)

			// Create indexes
			err = mongoDriver.EnsureIndexes(ctx)
			if err != nil {
				return fmt.Errorf("failed to ensure indexes: %w", err)
			}

			log.Info("Ensured indexes")

			ec, err := ethclient.Dial(cfg.rpcEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial rpc endpoint: %w", err)
			}

			networkID, err := ec.NetworkID(ctx)
			if err != nil {
				return fmt.Errorf("failed to get network id: %w", err)
			}

			hasProcessingStatus, err := mongoDriver.HasProcessingStatus(ctx, networkID.String())
			if err != nil {
				return fmt.Errorf("failed to check if processing status exists: %w", err)
			}

			log.Info("has processing status", "hasProcessingStatus", hasProcessingStatus)

			if !hasProcessingStatus {
				log.Info("no processing status found, inserting genesis block")

				genesisHeader, err := ec.HeaderByNumber(ctx, big.NewInt(0))
				if err != nil {
					return fmt.Errorf("failed to get genesis header: %w", err)
				}

				err = mongoDriver.InsertProcessingStatus(ctx, mongogolem.ProcessingStatus{
					Network:                  networkID.String(),
					LastProcessedBlockNumber: 0,
					LastProcessedBlockHash:   genesisHeader.Hash().String(),
				})
				if err != nil {
					return fmt.Errorf("failed to insert processing status: %w", err)
				}
			}

			processingStatus, err := mongoDriver.GetProcessingStatus(ctx, networkID.String())
			if err != nil {
				return fmt.Errorf("failed to get processing status: %w", err)
			}

			blockNumber := processingStatus.LastProcessedBlockNumber
			blockHash := processingStatus.LastProcessedBlockHash

			for blockWal, err := range wal.NewIterator(ctx, cfg.walDir, uint64(blockNumber)+1, common.HexToHash(blockHash), true) {
				if err != nil {
					return fmt.Errorf("failed to iterate over wal: %w", err)
				}

				err = func() (err error) {
					log.Info("processing block", "block", blockWal.BlockInfo.Number)

					// Create a session with a timeout for each block processing
					sessCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()

					session, err := client.StartSession()
					if err != nil {
						return fmt.Errorf("failed to start MongoDB session: %w", err)
					}
					defer session.EndSession(sessCtx)

					// Use WithTransaction to handle transactions
					_, err = session.WithTransaction(sessCtx, func(txCtx mongo.SessionContext) (interface{}, error) {
						for op, err := range blockWal.OperationsIterator {
							if err != nil {
								return nil, fmt.Errorf("failed to iterate over operations: %w", err)
							}

							switch {
							case op.Create != nil:
								log.Info("create", "entity", op.Create.EntityKey.Hex())

								// Convert string and numeric annotations to maps
								stringAnnotations := make(map[string]string)
								for _, annotation := range op.Create.StringAnnotations {
									stringAnnotations[annotation.Key] = annotation.Value
								}

								numericAnnotations := make(map[string]int64)
								for _, annotation := range op.Create.NumericAnnotations {
									numericAnnotations[annotation.Key] = int64(annotation.Value)
								}

								err = mongoDriver.InsertEntity(txCtx, mongogolem.Entity{
									Key:                op.Create.EntityKey.Hex(),
									ExpiresAt:          int64(op.Create.ExpiresAtBlock),
									Payload:            op.Create.Payload,
									StringAnnotations:  stringAnnotations,
									NumericAnnotations: numericAnnotations,
									OwnerAddress:       op.Create.Owner.Hex(),
								})
								if err != nil {
									return nil, fmt.Errorf("failed to insert entity: %w", err)
								}

							case op.Update != nil:
								log.Info("update", "entity", op.Update.EntityKey.Hex())

								// Get the existing entity to preserve the owner address
								existingEntity, err := mongoDriver.GetEntity(txCtx, op.Update.EntityKey.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to get existing entity: %w", err)
								}

								// Delete existing entity
								err = mongoDriver.DeleteEntity(txCtx, op.Update.EntityKey.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete entity before update: %w", err)
								}

								// Convert string and numeric annotations to maps
								stringAnnotations := make(map[string]string)
								for _, annotation := range op.Update.StringAnnotations {
									stringAnnotations[annotation.Key] = annotation.Value
								}

								numericAnnotations := make(map[string]int64)
								for _, annotation := range op.Update.NumericAnnotations {
									numericAnnotations[annotation.Key] = int64(annotation.Value)
								}

								// Insert updated entity
								err = mongoDriver.InsertEntity(txCtx, mongogolem.Entity{
									Key:                op.Update.EntityKey.Hex(),
									ExpiresAt:          int64(op.Update.ExpiresAtBlock),
									Payload:            op.Update.Payload,
									StringAnnotations:  stringAnnotations,
									NumericAnnotations: numericAnnotations,
									OwnerAddress:       existingEntity.OwnerAddress,
								})
								if err != nil {
									return nil, fmt.Errorf("failed to insert updated entity: %w", err)
								}

							case op.Delete != nil:
								log.Info("delete", "entity", op.Delete.Hex())

								err = mongoDriver.DeleteEntity(txCtx, op.Delete.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete entity: %w", err)
								}

							case op.Extend != nil:
								log.Info("extend BTL", "entity", op.Extend.EntityKey.Hex(), "from", op.Extend.OldExpiresAt, "to", op.Extend.NewExpiresAt)

								// Get the existing entity
								entity, err := mongoDriver.GetEntity(txCtx, op.Extend.EntityKey.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to get entity for BTL extension: %w", err)
								}

								// Update the entity's expiry time to the new value
								entity.ExpiresAt = int64(op.Extend.NewExpiresAt)

								// Delete and reinsert with updated expiry
								err = mongoDriver.DeleteEntity(txCtx, op.Extend.EntityKey.Hex())
								if err != nil {
									return nil, fmt.Errorf("failed to delete entity before BTL extension: %w", err)
								}

								err = mongoDriver.InsertEntity(txCtx, entity)
								if err != nil {
									return nil, fmt.Errorf("failed to insert entity with extended BTL: %w", err)
								}
							}

							log.Info("operation", "operation", op)
						}

						// Update processing status
						err = mongoDriver.UpdateProcessingStatus(txCtx, mongogolem.ProcessingStatus{
							Network:                  networkID.String(),
							LastProcessedBlockNumber: int64(blockWal.BlockInfo.Number),
							LastProcessedBlockHash:   blockWal.BlockInfo.Hash.String(),
						})
						if err != nil {
							return nil, fmt.Errorf("failed to update processing status: %w", err)
						}

						return nil, nil
					})

					return err
				}()

				if err != nil {
					return fmt.Errorf("failed to process block: %w", err)
				}
			}

			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Error("failed to run app", "error", err)
		os.Exit(1)
	}
}
