package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/signal"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/golem-base/etl/sqlite/sqlitegolem"
	"github.com/ethereum/go-ethereum/golem-base/wal"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

//go:embed sqlitegolem/schema.sql
var schema string

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := struct {
		dbFile      string
		walDir      string
		rpcEndpoint string
	}{}
	app := &cli.App{
		Name: "sqlite-etl",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "db",
				Usage:       "database file",
				EnvVars:     []string{"DB_FILE"},
				Destination: &cfg.dbFile,
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

			db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", cfg.dbFile))
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()

			var tableName string
			err = db.QueryRowContext(ctx, `
				SELECT name FROM sqlite_master 
				WHERE type='table' AND name='entities';
			`).Scan(&tableName)

			if err == sql.ErrNoRows {
				log.Info("could not find 'entities' table, applying schema")
				_, err := db.ExecContext(ctx, schema)
				if err != nil {
					return fmt.Errorf("failed to apply schema table: %w", err)
				}
			}

			autocommit := sqlitegolem.New(db)

			ec, err := ethclient.Dial(cfg.rpcEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial rpc endpoint: %w", err)
			}

			networkID, err := ec.NetworkID(ctx)
			if err != nil {
				return fmt.Errorf("failed to get network id: %w", err)
			}

			hasProcessingStatus, err := autocommit.HasProcessingStatus(ctx, networkID.String())
			if err != nil {
				return fmt.Errorf("failed to check if processing status exists: %w", err)
			}

			if !hasProcessingStatus {
				log.Info("no processing status found, inserting genesis block")

				genesisHeade, err := ec.HeaderByNumber(ctx, big.NewInt(0))
				if err != nil {
					return fmt.Errorf("failed to get genesis header: %w", err)
				}

				err = autocommit.InsertProcessingStatus(ctx, sqlitegolem.InsertProcessingStatusParams{
					Network:                  networkID.String(),
					LastProcessedBlockNumber: 0,
					LastProcessedBlockHash:   genesisHeade.Hash().String(),
				})
				if err != nil {
					return fmt.Errorf("failed to insert processing status: %w", err)
				}
			}

			processingStatus, err := autocommit.GetProcessingStatus(ctx, networkID.String())
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
					tx, err := db.BeginTx(ctx, nil)
					if err != nil {
						return fmt.Errorf("failed to begin transaction: %w", err)
					}

					defer func() {
						if err != nil {
							err = errors.Join(err, tx.Rollback())
						}
					}()

					txDB := sqlitegolem.New(tx)

					for op, err := range blockWal.OperationsIterator {
						if err != nil {
							return fmt.Errorf("failed to iterate over operations: %w", err)
						}

						switch {
						case op.Create != nil:
							log.Info("create", "entity", op.Create.EntityKey.Hex())
							err = txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
								Key:          op.Create.EntityKey.Hex(),
								ExpiresAt:    int64(op.Create.ExpiresAtBlock),
								Payload:      op.Create.Payload,
								OwnerAddress: op.Create.Owner.Hex(),
							})
							if err != nil {
								return fmt.Errorf("failed to insert entity: %w", err)
							}

							for _, annotation := range op.Create.NumericAnnotations {
								err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
									EntityKey:     op.Create.EntityKey.Hex(),
									AnnotationKey: annotation.Key,
									Value:         int64(annotation.Value),
								})
								if err != nil {
									return fmt.Errorf("failed to insert numeric annotation: %w", err)
								}
							}

							for _, annotation := range op.Create.StringAnnotations {
								err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
									EntityKey:     op.Create.EntityKey.Hex(),
									AnnotationKey: annotation.Key,
									Value:         annotation.Value,
								})
								if err != nil {
									return fmt.Errorf("failed to insert string annotation: %w", err)
								}
							}
						case op.Update != nil:
							existingEntity, err := txDB.GetEntity(ctx, op.Update.EntityKey.Hex())
							if err != nil {
								return fmt.Errorf("failed to get existing entity: %w", err)
							}

							txDB.DeleteEntity(ctx, op.Update.EntityKey.Hex())
							txDB.DeleteNumericAnnotations(ctx, op.Update.EntityKey.Hex())
							txDB.DeleteStringAnnotations(ctx, op.Update.EntityKey.Hex())

							txDB.InsertEntity(ctx, sqlitegolem.InsertEntityParams{
								Key:          op.Update.EntityKey.Hex(),
								ExpiresAt:    int64(op.Update.ExpiresAtBlock),
								Payload:      op.Update.Payload,
								OwnerAddress: existingEntity.OwnerAddress,
							})

							for _, annotation := range op.Update.NumericAnnotations {
								err = txDB.InsertNumericAnnotation(ctx, sqlitegolem.InsertNumericAnnotationParams{
									EntityKey:     op.Update.EntityKey.Hex(),
									AnnotationKey: annotation.Key,
									Value:         int64(annotation.Value),
								})
								if err != nil {
									return fmt.Errorf("failed to insert numeric annotation: %w", err)
								}
							}

							for _, annotation := range op.Update.StringAnnotations {
								err = txDB.InsertStringAnnotation(ctx, sqlitegolem.InsertStringAnnotationParams{
									EntityKey:     op.Update.EntityKey.Hex(),
									AnnotationKey: annotation.Key,
									Value:         annotation.Value,
								})
								if err != nil {
									return fmt.Errorf("failed to insert string annotation: %w", err)
								}
							}
						case op.Delete != nil:
							err = txDB.DeleteEntity(ctx, op.Delete.Hex())
							if err != nil {
								return fmt.Errorf("failed to delete entity: %w", err)
							}

							err = txDB.DeleteNumericAnnotations(ctx, op.Delete.Hex())
							if err != nil {
								return fmt.Errorf("failed to delete numeric annotations: %w", err)
							}

							err = txDB.DeleteStringAnnotations(ctx, op.Delete.Hex())
							if err != nil {
								return fmt.Errorf("failed to delete string annotations: %w", err)
							}
						case op.Extend != nil:
							log.Info("extend BTL", "entity", op.Extend.EntityKey.Hex())

							// Update the entity with the new expiry time
							err = txDB.UpdateEntityExpiresAt(ctx, sqlitegolem.UpdateEntityExpiresAtParams{
								ExpiresAt: int64(op.Extend.NewExpiresAt),
								Key:       op.Extend.EntityKey.Hex(),
							})
							if err != nil {
								return fmt.Errorf("failed to extend entity BTL: %w", err)
							}
						}

						log.Info("operation", "operation", op)
					}

					return tx.Commit()

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
