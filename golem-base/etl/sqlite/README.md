# SQLite ETL

This program implements an Extract, Transform, Load (ETL) process that processes blockchain data from the Write-Ahead Log (WAL) written by the Golem Base extension of op-geth. The WAL contains entity operations (create, update, delete) and their associated annotations, which this program processes and stores in a SQLite database.

## Features

- Processes blockchain data from Golem Base WAL files
- Stores entity data and annotations in SQLite
- Supports numeric and string annotations for entities
- Handles entity lifecycle operations (create, update, delete)
- Supports BTL extension for entities
- Maintains processing status to track progress

## Requirements

- Go 1.x
- SQLite3
- Access to an op-geth RPC endpoint
- op-geth configured with Golem Base extension

## op-geth Configuration

To enable WAL writing in op-geth with Golem Base extension, you need to:

1. Start op-geth with the `--golembase.writeaheadlog` flag pointing to your desired WAL directory:
```bash
op-geth --golembase.writeaheadlog /path/to/wal/directory
```

2. Make sure the WAL directory exists and is writable by the op-geth process.

The WAL directory will contain files that record all entity operations processed by the Golem Base extension. These files are what the SQLite ETL program processes.

## Configuration

The program requires the following configuration parameters:

- `--db`: SQLite database file path (required)
- `--wal`: Directory containing the Write-Ahead Log files (required)
- `--rpc-endpoint`: URL of the op-geth RPC endpoint (required)

These can be provided via command line flags or environment variables:
- `DB_FILE`
- `WAL_DIR`
- `RPC_ENDPOINT`

## Usage

```bash
sqlite-etl --db golembase.db --wal ./wal --rpc-endpoint http://localhost:8545
```

## Database Structure

The program uses a SQLite database with the following main tables:

- `entities`: Stores the main entity data and annotations
- `processing_status`: Tracks the last processed block

Entity records in SQLite include:
- `key`: The entity key (primary key)
- `content`: The entity payload
- `stringAnnotations`: String annotations for the entity
- `numericAnnotations`: Numeric annotations for the entity
- `created_at`: Timestamp when the entity was created
- `updated_at`: Timestamp when the entity was last updated
- `expires_at`: Expiration time for the entity (if applicable)
- `owner_address`: The Ethereum address of the entity owner (hex string)

The following indexes are created for efficient querying:
- `idx_entities_owner_address`: Index on the owner's Ethereum address
- `expires_at`: Index for BTL queries
- `stringAnnotations`: Index for string annotation queries
- `numericAnnotations`: Index for numeric annotation queries

## Processing Flow

1. Connects to the op-geth RPC endpoint and SQLite database
2. Checks for existing processing status
3. If no status exists, initializes with genesis block
4. Processes WAL files sequentially
5. For each block:
   - Processes all operations (create, update, delete, extend BTL)
   - Handles entity data and annotations
   - For BTL extensions, updates the expiration block of the entity
   - Updates processing status
6. Uses SQLite transactions to ensure data consistency

## Supported Operations

The ETL process handles the following operations from the WAL:

1. **Create**: Creates a new entity with associated annotations
2. **Update**: Updates an existing entity's payload and annotations
3. **Delete**: Removes an entity and its annotations from the database
4. **Extend BTL**: Updates an entity's expiration block without modifying its payload or annotations

## Error Handling

- Graceful shutdown on interrupt signals
- Transaction rollback on processing errors
- Detailed error logging
- Automatic retry mechanisms using backoff strategies
- Robust error reporting
