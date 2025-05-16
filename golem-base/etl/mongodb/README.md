# MongoDB ETL

This program implements an Extract, Transform, Load (ETL) process that processes blockchain data from the Write-Ahead Log (WAL) written by the Golem Base extension of op-geth. The WAL contains entity operations (create, update, delete) and their associated annotations, which this program processes and stores in a MongoDB database.

## Features

- Processes blockchain data from Golem Base WAL files
- Stores entity data and annotations in MongoDB
- Supports numeric and string annotations for entities
- Handles entity lifecycle operations (create, update, delete)
- Supports BTL extension operations for entities
- Maintains processing status to track progress

## Requirements

- Go 1.x
- MongoDB
- Access to an op-geth RPC endpoint
- op-geth configured with Golem Base extension

## op-geth Configuration

To enable WAL writing in op-geth with Golem Base extension, you need to:

1. Start op-geth with the `--golembase.writeaheadlog` flag pointing to your desired WAL directory:
```bash
op-geth --golembase.writeaheadlog /path/to/wal/directory
```

2. Make sure the WAL directory exists and is writable by the op-geth process.

The WAL directory will contain files that record all entity operations processed by the Golem Base extension. These files are what the MongoDB ETL program processes.

## MongoDB Transaction Support

This ETL uses MongoDB transactions to ensure data consistency when processing block operations. MongoDB transactions require a replica set deployment, as transactions are not supported on standalone MongoDB instances.

### Setting Up a MongoDB Replica Set

For development purposes, you can set up a single-node replica set:

```bash
# Start MongoDB with replica set enabled
mongod --replSet rs0 --dbpath /path/to/data/directory

# Connect to MongoDB and initiate the replica set
mongo
> rs.initiate()
```

For production environments, a proper multi-node replica set is recommended for high availability.

## Configuration

The program requires the following configuration parameters:

- `--mongo-url`: MongoDB connection string (required)
- `--db-name`: MongoDB database name (required)
- `--wal`: Directory containing the Write-Ahead Log files (required)
- `--rpc-endpoint`: URL of the op-geth RPC endpoint (required)

These can be provided via command line flags or environment variables:
- `MONGO_URI`
- `DB_NAME`
- `WAL_DIR`
- `RPC_ENDPOINT`

## Usage

```bash
mongodb-etl --mongo-url mongodb://localhost:27017?replicaSet=rs0 --db-name golembase --wal ./wal --rpc-endpoint http://localhost:8545
```

## Database Structure

The program uses a MongoDB database with the following main collections:

- `entities`: Stores the main entity data and annotations
- `processing_status`: Tracks the last processed block

Entity documents in MongoDB include:
- `_id`: The entity key
- `content`: The entity payload
- `content_json`: JSON-deserialized payload (if payload is valid JSON)
- `stringAnnotations`: String annotations for the entity
- `numericAnnotations`: Numeric annotations for the entity
- `created_at`: Timestamp when the entity was created
- `updated_at`: Timestamp when the entity was last updated
- `expires_at`: Expiration time for the entity (block number)
- `owner_address`: The Ethereum address of the entity owner (hex string)

The following indexes are created for efficient querying:
- `owner_address`: Index on the owner's Ethereum address
- `expires_at`: Index for BTL queries
- `stringAnnotations.$**`: Wildcard index for string annotation queries
- `numericAnnotations.$**`: Wildcard index for numeric annotation queries

## Processing Flow

1. Connects to the op-geth RPC endpoint and MongoDB
2. Checks for existing processing status
3. If no status exists, initializes with genesis block
4. Processes WAL files sequentially
5. For each block:
   - Processes all operations (create, update, delete, extend BTL)
   - Handles entity data and annotations
   - For BTL extensions, updates the entity's expiration block number
   - Updates processing status
6. Uses MongoDB transactions to ensure data consistency

## BTL Extension

The ETL supports entity Blocks-To-Live (BTL) extension operations:

- When an entity's BTL is extended in Golembase, the extension is recorded in the WAL
- The ETL processes these extension operations, updating the entity's `expires_at` field
- The original owner is preserved during BTL extension
- BTL extension operations are processed atomically within MongoDB transactions
- The ETL maintains consistency between Golembase and MongoDB for entity expiration

## Error Handling

- Graceful shutdown on interrupt signals
- Transaction rollback on processing errors
- Detailed error logging
- Automatic retry mechanisms using backoff strategies
- Robust error reporting 