package main_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/ethereum/go-ethereum/golem-base/etl/mongodb/etlworld"
	"github.com/ethereum/go-ethereum/golem-base/etl/mongodb/mongogolem"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/holiman/uint256"
	"github.com/spf13/pflag" // godog v0.11.0 and later
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var opts = godog.Options{
	Output:      colors.Uncolored(os.Stdout),
	Format:      "progress",
	Strict:      true,
	Concurrency: 4,

	Paths: []string{"features"},
}

func init() {
	godog.BindCommandLineFlags("godog.", &opts)

	if os.Getenv("CUCUMBER_WIP_ONLY") == "true" {
		opts.Tags = "@wip"
		opts.Concurrency = 1
		opts.Format = "pretty"
	}
}

func compileGeth() (string, func(), error) {
	td, err := os.MkdirTemp("", "golem-base")
	if err != nil {
		panic(fmt.Errorf("failed to create temp dir: %w", err))
	}

	gethBinaryPath := filepath.Join(td, "geth")

	cmd := exec.Command("go", "build", "-o", gethBinaryPath, "../../../cmd/geth")
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to compile geth: %w\n%s", err, out.String())
	}

	return gethBinaryPath, func() {
		os.RemoveAll(td)
	}, nil
}

func compileMongodbETL() (string, func(), error) {
	td, err := os.MkdirTemp("", "mongodb-etl")
	if err != nil {
		panic(fmt.Errorf("failed to create temp dir: %w", err))
	}

	mongodbETLBinaryPath := filepath.Join(td, "mongodb-etl")

	cmd := exec.Command("go", "build", "-o", mongodbETLBinaryPath, ".")
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to compile mongodb-etl: %w\n%s", err, out.String())
	}

	return mongodbETLBinaryPath, func() {
		os.RemoveAll(td)
	}, nil
}

func TestMain(m *testing.M) {
	pflag.Parse()
	opts.Paths = pflag.Args()

	gethPath, cleanupCompiled, err := compileGeth()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to compile geth: %w", err))
	}

	mongodbETLPath, cleanupCompiledMongodbETL, err := compileMongodbETL()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to compile geth: %w", err))
	}

	suite := godog.TestSuite{
		Name: "cucumber",
		ScenarioInitializer: func(sctx *godog.ScenarioContext) {
			InitializeScenario(sctx)
			sctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {

				world, err := etlworld.NewETLWorld(ctx, gethPath, mongodbETLPath)
				if err != nil {
					return ctx, fmt.Errorf("failed to start geth instance: %w", err)
				}

				timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

				sctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
					world.Shutdown()
					cancel()
					return ctx, world.AddLogsToTestError(err)
				})

				return etlworld.WithWorld(timeoutCtx, world), nil

			})

		},
		// ScenarioInitializer:  InitializeScenario,
		Options: &opts,
	}

	status := suite.Run()

	// // Optional: Run `testing` package's logic besides godog.
	// if st := m.Run(); st > status {
	// 	status = st
	// }

	cleanupCompiled()
	cleanupCompiledMongodbETL()

	os.Exit(status)
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^A running ETL to Mongodb$`, aRunningETLToMongodb)
	ctx.Step(`^A running Golembase node with WAL enabled$`, aRunningGolembaseNodeWithWALEnabled)
	ctx.Step(`^I create a new entity in Golebase$`, iCreateANewEntityInGolebase)
	ctx.Step(`^the annotations of the entity should be existing in the Mongodb database$`, theAnnotationsOfTheEntityShouldBeExistingInTheMongodbDatabase)
	ctx.Step(`^the entity should be created in the Mongodb database$`, theEntityShouldBeCreatedInTheMongodbDatabase)
	ctx.Step(`^an existing entity in the Mongodb database$`, anExistingEntityInTheMongodbDatabase)
	ctx.Step(`^the annotations of the entity should be updated in the Mongodb database$`, theAnnotationsOfTheEntityShouldBeUpdatedInTheMongodbDatabase)
	ctx.Step(`^the entity should be updated in the Mongodb database$`, theEntityShouldBeUpdatedInTheMongodbDatabase)
	ctx.Step(`^update the entity in Golembase$`, updateTheEntityInGolembase)
	ctx.Step(`^delete the entity in Golembase$`, deleteTheEntityInGolembase)
	ctx.Step(`^the annotations of the entity should be deleted in the SQLite database$`, theAnnotationsOfTheEntityShouldBeDeletedInTheSQLiteDatabase)
	ctx.Step(`^the entity should be deleted in the Mongodb database$`, theEntityShouldBeDeletedInTheMongodbDatabase)
	ctx.Step(`^I create an entity with a JSON payload to the Golembase$`, iCreateAnEntityWithAJSONPayloadToTheGolembase)
	ctx.Step(`^the PayloadAsJSON in the Mongodb database should be populated$`, thePayloadAsJSONInTheMongodbDatabaseShouldBePopulated)
	ctx.Step(`^the owner address should be stored in the Mongodb database$`, theOwnerAddressShouldBeStoredInTheMongodbDatabase)
	ctx.Step(`^the owner address should be preserved in the Mongodb database$`, theOwnerAddressShouldBePreservedInTheMongodbDatabase)
	ctx.Step(`^a new entity in Golebase$`, aNewEntityInGolebase)
	ctx.Step(`^update the BTL of the entity in Golembase$`, updateTheBTLOfTheEntityInGolembase)
	ctx.Step(`^the BTL of the entity should be extended in the Mongodb database$`, theBTLOfTheEntityShouldBeExtendedInTheMongodbDatabase)
}

func aRunningETLToMongodb() error {
	// Started when the world is created
	return nil
}

func aRunningGolembaseNodeWithWALEnabled() error {
	// Started when the world is created
	return nil
}

func iCreateANewEntityInGolebase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	_, err := w.CreateEntity(ctx,
		1000,
		[]byte("test"),
		[]entity.StringAnnotation{
			{
				Key:   "stringTest",
				Value: "stringTest",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "numericTest",
				Value: 1234567890,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil
}

func theEntityShouldBeCreatedInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	entityKey := w.CreatedEntityKey

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {
		collection := mongo.Collections().Entities
		filter := bson.M{"_id": entityKey.Hex()}

		bo := backoff.WithContext(backoff.NewConstantBackOff(100*time.Millisecond), timeoutCtx)

		return backoff.Retry(
			func() error {

				count, err := collection.CountDocuments(timeoutCtx, filter)
				if err != nil {
					return fmt.Errorf("failed to count documents: %w", err)
				}
				if count != 1 {
					return fmt.Errorf("expected 1 document, got %d", count)
				}
				return nil
			},
			bo,
		)
	})

}

type Entity struct {
	Key                string            `bson:"_id"`
	ExpiresAt          int64             `bson:"expires_at"`
	Payload            []byte            `bson:"content"`
	PayloadAsJSON      interface{}       `bson:"content_json,omitempty"`
	StringAnnotations  map[string]string `bson:"stringAnnotations,omitempty"`
	NumericAnnotations map[string]int64  `bson:"numericAnnotations,omitempty"`
	CreatedAt          time.Time         `bson:"created_at"`
	UpdatedAt          time.Time         `bson:"updated_at"`
	OwnerAddress       string            `bson:"owner_address,omitempty"`
}

func theAnnotationsOfTheEntityShouldBeExistingInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	entityKey := w.CreatedEntityKey

	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {
		collection := mongo.Collections().Entities
		filter := bson.M{"_id": entityKey.Hex()}
		res := collection.FindOne(ctx, filter)
		if res.Err() != nil {
			return fmt.Errorf("failed to find entity: %w", res.Err())
		}

		var entity Entity
		err := res.Decode(&entity)
		if err != nil {
			return fmt.Errorf("failed to decode entity: %w", err)
		}
		if len(entity.StringAnnotations) != 1 {
			return fmt.Errorf("expected 1 string annotation, got %d", len(entity.StringAnnotations))
		}
		if len(entity.NumericAnnotations) != 1 {
			return fmt.Errorf("expected 1 numeric annotation, got %d", len(entity.NumericAnnotations))
		}
		return nil
	})
}

func anExistingEntityInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	_, err := w.CreateEntity(ctx,
		1000,
		[]byte(`{"test": "value", "number": 123}`),
		[]entity.StringAnnotation{
			{
				Key:   "stringTest",
				Value: "stringTest",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "numericTest",
				Value: 1234567890,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil
}

func theAnnotationsOfTheEntityShouldBeUpdatedInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	entityKey := w.CreatedEntityKey

	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {
		collection := mongo.Collections().Entities
		filter := bson.M{"_id": entityKey.Hex()}
		res := collection.FindOne(ctx, filter)
		if res.Err() != nil {
			return fmt.Errorf("failed to find entity: %w", res.Err())
		}

		var entity Entity
		err := res.Decode(&entity)
		if err != nil {
			return fmt.Errorf("failed to decode entity: %w", err)
		}
		if len(entity.StringAnnotations) != 1 {
			return fmt.Errorf("expected 1 string annotation, got %d", len(entity.StringAnnotations))
		}

		if string(entity.Payload) != `test2` {
			return fmt.Errorf("expected payload to be test2, got %s", string(entity.Payload))
		}

		if entity.StringAnnotations["stringTest2"] != "stringTest2" {
			return fmt.Errorf("expected string annotation to be stringTest2, got %s", entity.StringAnnotations["stringTest2"])
		}

		if len(entity.NumericAnnotations) != 1 {
			return fmt.Errorf("expected 1 numeric annotation, got %d", len(entity.NumericAnnotations))
		}
		if entity.NumericAnnotations["numericTest2"] != 12345678901 {
			return fmt.Errorf("expected numeric annotation to be 12345678901, got %d", entity.NumericAnnotations["numericTest2"])
		}

		return nil
	})
}

func theEntityShouldBeUpdatedInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	entityKey := w.CreatedEntityKey

	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		var entity Entity
		var lastErr error

		err := backoff.Retry(func() error {
			collection := mongo.Collections().Entities
			filter := bson.M{"_id": entityKey.Hex()}
			res := collection.FindOne(timeoutCtx, filter)
			if res.Err() != nil {
				lastErr = fmt.Errorf("failed to find entity: %w", res.Err())
				return lastErr
			}

			err := res.Decode(&entity)
			if err != nil {
				lastErr = fmt.Errorf("failed to decode entity: %w", err)
				return lastErr
			}

			if string(entity.Payload) != `test2` {
				lastErr = fmt.Errorf("expected payload to be test2, got %s", string(entity.Payload))
				return lastErr
			}

			if entity.StringAnnotations["stringTest2"] != "stringTest2" {
				lastErr = fmt.Errorf("expected string annotation to be stringTest2, got %s", entity.StringAnnotations["stringTest2"])
				return lastErr
			}

			if entity.NumericAnnotations["numericTest2"] != 12345678901 {
				lastErr = fmt.Errorf("expected numeric annotation to be 12345678901, got %d", entity.NumericAnnotations["numericTest2"])
				return lastErr
			}

			return nil
		}, backoff.WithContext(backoff.NewConstantBackOff(200*time.Millisecond), timeoutCtx))

		if err != nil {
			return lastErr
		}

		return nil
	})

}

func updateTheEntityInGolembase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	_, err := w.UpdateEntity(ctx,
		w.CreatedEntityKey,
		999,
		[]byte("test2"),
		[]entity.StringAnnotation{
			{
				Key:   "stringTest2",
				Value: "stringTest2",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "numericTest2",
				Value: 12345678901,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}
	return nil

}

func deleteTheEntityInGolembase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	_, err := w.DeleteEntity(ctx, w.CreatedEntityKey)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}
	return nil
}

func theAnnotationsOfTheEntityShouldBeDeletedInTheSQLiteDatabase() error {
	return godog.ErrPending
}

func theEntityShouldBeDeletedInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {

		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		bo := backoff.WithContext(backoff.NewConstantBackOff(200*time.Millisecond), ctx)

		return backoff.Retry(
			func() error {

				collection := mongo.Collections().Entities
				filter := bson.M{"_id": w.CreatedEntityKey.Hex()}
				cur, err := collection.Find(timeoutCtx, filter)
				if err != nil {
					return fmt.Errorf("failed to find entity: %w", err)
				}
				defer cur.Close(ctx)
				for cur.Next(timeoutCtx) {
					return fmt.Errorf("expected 0 entities, got 1")
				}
				return nil
			},
			bo,
		)
	})
}

func thePayloadAsJSONInTheMongodbDatabaseShouldBePopulated(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	entityKey := w.CreatedEntityKey

	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {
		collection := mongo.Collections().Entities
		filter := bson.M{"_id": entityKey.Hex()}
		res := collection.FindOne(ctx, filter)
		if res.Err() != nil {
			return fmt.Errorf("failed to find entity: %w", res.Err())
		}

		var entity Entity
		err := res.Decode(&entity)
		if err != nil {
			return fmt.Errorf("failed to decode entity: %w", err)
		}

		if entity.PayloadAsJSON == nil {
			return fmt.Errorf("expected PayloadAsJSON to be populated, but it was nil")
		}

		var testValue interface{}
		var found bool

		switch payload := entity.PayloadAsJSON.(type) {
		case map[string]interface{}:
			testValue, found = payload["test"]
		case primitive.D:
			for _, elem := range payload {
				if elem.Key == "test" {
					testValue = elem.Value
					found = true
					break
				}
			}
		default:
			return fmt.Errorf("expected PayloadAsJSON to be a map or primitive.D, got %T", entity.PayloadAsJSON)
		}

		if !found || testValue != "value" {
			return fmt.Errorf("expected PayloadAsJSON to have test=value, got %v", testValue)
		}

		return nil
	})
}

func iCreateAnEntityWithAJSONPayloadToTheGolembase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	_, err := w.CreateEntity(ctx,
		1000,
		[]byte(`{"test": "value", "number": 123}`),
		[]entity.StringAnnotation{
			{
				Key:   "stringTest",
				Value: "stringTest",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "numericTest",
				Value: 1234567890,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil
}

func theOwnerAddressShouldBeStoredInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	entityKey := w.CreatedEntityKey

	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {
		collection := mongo.Collections().Entities
		filter := bson.M{"_id": entityKey.Hex()}
		res := collection.FindOne(ctx, filter)
		if res.Err() != nil {
			return fmt.Errorf("failed to find entity: %w", res.Err())
		}

		var entity Entity
		err := res.Decode(&entity)
		if err != nil {
			return fmt.Errorf("failed to decode entity: %w", err)
		}

		if entity.OwnerAddress == "" {
			return fmt.Errorf("expected owner address to be stored, but it was empty")
		}

		return nil
	})
}

func theOwnerAddressShouldBePreservedInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	entityKey := w.CreatedEntityKey

	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {
		collection := mongo.Collections().Entities
		filter := bson.M{"_id": entityKey.Hex()}
		res := collection.FindOne(ctx, filter)
		if res.Err() != nil {
			return fmt.Errorf("failed to find entity: %w", res.Err())
		}

		var entity Entity
		err := res.Decode(&entity)
		if err != nil {
			return fmt.Errorf("failed to decode entity: %w", err)
		}

		if entity.OwnerAddress == "" {
			return fmt.Errorf("expected owner address to be preserved, but it was empty")
		}

		return nil
	})
}

func aNewEntityInGolebase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	receipt, err := w.CreateEntity(ctx,
		1000,
		[]byte(`{"test": "value", "number": 123}`),
		[]entity.StringAnnotation{
			{
				Key:   "stringTest",
				Value: "stringTest",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "numericTest",
				Value: 1234567890,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	initialExpiresAtBlock := uint256.NewInt(0).SetBytes(receipt.Logs[0].Data)
	w.InitialExpiresAtBlock = initialExpiresAtBlock.Uint64()

	return nil

}

func updateTheBTLOfTheEntityInGolembase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)

	_, err := w.ExtendBTL(ctx, w.CreatedEntityKey, 500) // Extend by 500 blocks
	if err != nil {
		return fmt.Errorf("failed to extend BTL: %w", err)
	}
	return nil
}

func theBTLOfTheEntityShouldBeExtendedInTheMongodbDatabase(ctx context.Context) error {
	w := etlworld.GetWorld(ctx)
	entityKey := w.CreatedEntityKey

	expectedExpiresAt := w.InitialExpiresAtBlock + w.BTLExtendedBy

	return w.AccessMongodb(func(mongo *mongogolem.MongoGolem) error {
		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		bo := backoff.WithContext(backoff.NewConstantBackOff(200*time.Millisecond), timeoutCtx)

		return backoff.Retry(func() error {
			collection := mongo.Collections().Entities
			filter := bson.M{"_id": entityKey.Hex()}
			res := collection.FindOne(timeoutCtx, filter)
			if res.Err() != nil {
				return fmt.Errorf("failed to find entity: %w", res.Err())
			}

			var entity Entity
			err := res.Decode(&entity)
			if err != nil {
				return fmt.Errorf("failed to decode entity: %w", err)
			}

			if uint64(entity.ExpiresAt) != expectedExpiresAt {
				return fmt.Errorf("expected expires_at to be %d, got %d", expectedExpiresAt, entity.ExpiresAt)
			}

			return nil
		}, bo)
	})
}
