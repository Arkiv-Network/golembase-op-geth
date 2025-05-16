package etlworld

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/etl/mongodb/mongogolem"
	"github.com/ethereum/go-ethereum/golem-base/testutil"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ETLWorld is an environment for integration testing the MongoDB ETL.
type ETLWorld struct {
	*testutil.World
	mongoContainer        *mongodb.MongoDBContainer
	mongoURI              string
	mongoClient           *mongo.Client
	mongoDatabase         *mongo.Database
	mongoDriver           *mongogolem.MongoGolem
	mongoETLBinaryPath    string
	etlProcess            *etlProcess
	dbName                string
	InitialExpiresAtBlock uint64
	BTLExtendedBy         uint64
}

// NewETLWorld creates a new ETL world for testing with MongoDB.
func NewETLWorld(
	ctx context.Context,
	gethPath string,
	mongoETLPath string,
) (*ETLWorld, error) {
	world, err := testutil.NewWorld(ctx, gethPath)
	if err != nil {
		return nil, err
	}

	// Start MongoDB container
	mongoContainer, err := mongodb.Run(ctx, "mongo:6.0", mongodb.WithReplicaSet("rs0"))
	if err != nil {
		return nil, fmt.Errorf("failed to start MongoDB container: %w", err)
	}

	// Get MongoDB connection URI
	mongoURI, err := mongoContainer.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get MongoDB connection string: %w", err)
	}

	// Create MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	// Create test database
	dbName := "golem_test"
	db := client.Database(dbName)
	driver := mongogolem.New(db)

	// Create MongoDB indexes
	if err := driver.EnsureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	// Start ETL process with MongoDB configuration
	etlProcess, err := startETLProcess(
		ctx,
		mongoETLPath,
		world.GethInstance.WALDir,
		world.GethInstance.RPCEndpoint,
		mongoURI,
		dbName,
	)
	if err != nil {
		// Clean up MongoDB resources if ETL fails to start
		client.Disconnect(ctx)
		mongoContainer.Terminate(ctx)
		return nil, err
	}

	e := &ETLWorld{
		World:              world,
		mongoContainer:     mongoContainer,
		mongoURI:           mongoURI,
		mongoClient:        client,
		mongoDatabase:      db,
		mongoDriver:        driver,
		mongoETLBinaryPath: mongoETLPath,
		etlProcess:         etlProcess,
		dbName:             dbName,
	}

	return e, nil
}

// AddLogsToTestError enhances an error with ETL logs to aid in debugging.
func (w *ETLWorld) AddLogsToTestError(err error) error {
	if err == nil {
		return nil
	}

	err = fmt.Errorf("%w\n\nETL Logs:\n%s", err, w.etlProcess.output.String())

	return w.World.AddLogsToTestError(err)
}

// Shutdown cleans up all resources used by the ETL world.
func (w *ETLWorld) Shutdown() {
	ctx := context.Background()

	// First stop the ETL process
	if w.etlProcess != nil && w.etlProcess.cleanup != nil {
		w.etlProcess.cleanup()
	}

	// Disconnect MongoDB client
	if w.mongoClient != nil {
		w.mongoClient.Disconnect(ctx)
	}

	// Terminate MongoDB container
	if w.mongoContainer != nil {
		w.mongoContainer.Terminate(ctx)
	}

	// Finally shutdown the Ethereum node
	if w.World != nil {
		w.World.Shutdown()
	}
}

// GetMongoDriver returns the MongoDB driver for direct database operations
func (w *ETLWorld) GetMongoDriver() *mongogolem.MongoGolem {
	return w.mongoDriver
}

// ExtendBTL extends the BTL of an entity in the Golembase network.
func (w *ETLWorld) ExtendBTL(ctx context.Context, key common.Hash, numberOfBlocks uint64) (*types.Receipt, error) {
	w.BTLExtendedBy = numberOfBlocks
	return w.World.ExtendBTL(ctx, key, numberOfBlocks)
}
