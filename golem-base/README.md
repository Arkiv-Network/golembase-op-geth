# Golem Base Storage

Golem Base provides a robust storage layer with the following characteristics:

- **Transaction-Based Storage Mutations**: All changes to storage are executed through secure transaction submissions
- **RLP-Encoded Operations**: Each transaction contains a list of operations encoded using RLP (Recursive Length Prefix) for efficient data serialization
- **Core Operation Types**:
  - CREATE: Establish new storage entries with configurable block-to-live (BTL)
  - UPDATE: Modify existing storage entries, including payload and annotations
  - DELETE: Remove storage entries completely from the system
- **Automatic Expiration**: Each block includes a housekeeping transaction that automatically removes all entities that have reached their expiration block, ensuring storage efficiency

## Format of the Storage transaction

The Golem Base Storage transaction is a usual Ethereum transaction that is sent to a special address (`0x0000000000000000000000000000000060138453`).
Such a transaction is processed by the Golem Base subsystem to execute the storage operations contained in the Data field.

### Transaction Data

The transaction data field contains a StorageTransaction structure encoded using RLP. This structure consists of:

- `Create`: A list of Create operations, each containing:
  - `BTL`: Blocks-to-live in blocks, current block time of Optimism is 2 seconds.
  - `Payload`: The actual data to be stored
  - `StringAnnotations`: Key-value pairs with string values for indexing
  - `NumericAnnotations`: Key-value pairs with numeric values for indexing

- `Update`: A list of Update operations, each containing:
  - `EntityKey`: The key of the entity to update
  - `BTL`: New blocks-to-live in blocks
  - `Payload`: New data to replace existing payload
  - `StringAnnotations`: New string annotations
  - `NumericAnnotations`: New numeric annotations

- `Delete`: A list of entity keys (common.Hash) to be removed from storage

- `Extend`: A list of ExtendBTL operations, each containing:
  - `EntityKey`: The key of the entity to extend BTL for
  - `NumberOfBlocks`: Number of blocks to extend the BTL by

The transaction is atomic - all operations succeed or the entire transaction fails. Entity keys for Create operations are derived from the transaction hash, payload content, and operation index, making it unique across the whole blockchain. Annotations enable efficient querying of stored data through specialized indexes.

### Emitted Logs

When storage transactions are executed, the system emits logs to track entity lifecycle events:

- **GolemBaseStorageEntityCreated**: Emitted when a new entity is created
  - Event signature: `GolemBaseStorageEntityCreated(bytes32 entityKey, uint256 expirationBlock)`
  - Event topic: `0xce4b4ad6891d716d0b1fba2b4aeb05ec20edadb01df512263d0dde423736bbb9`
  - Topics: `[GolemBaseStorageEntityCreated, entityKey]`
  - Data: Contains the expiration block number

- **GolemBaseStorageEntityUpdated**: Emitted when an entity is updated
  - Event signature: `GolemBaseStorageEntityUpdated(bytes32 entityKey, uint256 newExpirationBlock)`
  - Event topic: `0xf371f40aa6932ad9dacbee236e5f3b93d478afe3934b5cfec5ea0d800a41d165`
  - Topics: `[GolemBaseStorageEntityUpdated, entityKey]`
  - Data: Contains the new expiration block number

- **GolemBaseStorageEntityDeleted**: Emitted when an entity is deleted
  - Event signature: `GolemBaseStorageEntityDeleted(bytes32 entityKey)`
  - Event topic: `0x0297b0e6eaf1bc2289906a8123b8ff5b19e568a60d002d47df44f8294422af93`
  - Topics: `[GolemBaseStorageEntityDeleted, entityKey]`
  - Data: Empty

- **GolemBaseStorageEntityBTLExtended**: Emitted when an entity's BTL is extended
  - Event signature: `GolemBaseStorageEntityBTLExtended(bytes32 entityKey, uint256 oldExpirationBlock, uint256 newExpirationBlock)`
  - Event topic: `0x59e1f1f7ff9674612ab2457a6a68a8edd1360241e739f09fae1082ea4498f230`
  - Topics: `[GolemBaseStorageEntityBTLExtended, entityKey]`
  - Data: Contains both the old and new expiration block numbers

These logs enable efficient tracking of storage changes and can be used by applications to monitor entity lifecycle events. The event signatures are defined as keccak256 hashes of their respective function signatures.

## Housekeeping Transaction

The Golem Base system includes an automatic housekeeping mechanism that runs during block processing to manage entity lifecycle. This process:

1. **Expires Entities**: At each block, the system identifies and removes entities whose BTL has expired
2. **Cleans Up Indexes**: When entities are deleted, their annotation indexes are automatically updated
3. **Emits Deletion Logs**: For each expired entity, a `GolemBaseStorageEntityDeleted` event is emitted

The housekeeping process is executed automatically as part of block processing, ensuring that storage remains clean and that expired data is properly removed from the system. This helps maintain system performance and ensures that temporary data doesn't persist beyond its intended lifetime.

The implementation uses a specialized index that tracks which entities expire at which block number, allowing for efficient cleanup without having to scan the entire storage space.

## JSON-RPC Namespace and Methods

The API methods are accessible through the following JSON-RPC endpoints:

- `golembase_getStorageValue`: Retrieves payload data for a given hash key
- `golembase_getEntityMetaData`: Retrieves the complete entity data including payload, BTL, and annotations for a given hash key
- `golembase_getEntitiesToExpireAtBlock`: Returns entities scheduled to expire at a specific block
- `golembase_getEntitiesForStringAnnotationValue`: Finds entities with matching string annotations
- `golembase_getEntitiesForNumericAnnotationValue`: Finds entities with matching numeric annotations
- `golembase_queryEntities`: Executes queries with a custom query language
- `golembase_getEntityCount`: Returns the total number of entities in storage
- `golembase_getAllEntityKeys`: Returns all entity keys currently in storage
- `golembase_getEntitiesOfOwner`: Returns all entity keys owned by a specific address

## API Functionality

This JSON-RPC API provides several capabilities:

1. **Storage Access**
   - `getStorageValue`: Retrieves payload data for a given hash key
   - `getEntityMetaData`: Retrieves complete entity data including payload, BTL, owner Ethereum address and annotations

2. **Entity Queries**
   - `getEntitiesToExpireAtBlock`: Returns entities scheduled to expire at a specific block
   - `getEntitiesForStringAnnotationValue`: Finds entities with matching string annotations
   - `getEntitiesForNumericAnnotationValue`: Finds entities with matching numeric annotations
   - `getEntityCount`: Returns the total number of entities in storage
   - `getAllEntityKeys`: Returns all entity keys currently in storage
   - `getEntitiesOfOwner`: Returns all entity keys owned by a specific Ethereum address

3. **Query Language Support**
   - `queryEntities`: Executes queries with a custom query language, returning structured results
     - Supports equality comparisons for both string and numeric annotations (e.g., `name = "test"` or `age = 123`)
     - Logical operators for complex queries:
       - AND operator: `&&` (e.g., `name = "test" && age = 30`)
       - OR operator: `||` (e.g., `status = "active" || status = "pending"`)
     - Parentheses for grouping expressions and controlling precedence (e.g., `(type = "document" || type = "image") && status = "approved"`)
     - String values must be enclosed in double quotes, with escape sequences for special characters
     - Numeric values are represented as unsigned integers
     - Returns an array of `SearchResult` objects containing:
       - `Key`: The entity's unique hash identifier
       - `Value`: The entity's payload data

## Development Environment and CLI Usage

### Running the Development Environment

Golem Base provides a development environment that includes all necessary services to work with the system. To start the development environment run the development script:
   ```
   ./golem-base/run_dev.sh
   ```

This will start all required services defined in the Procfile, including:
- Geth node in dev mode with Golem Base support
- SQLite ETL (Extract, Transform, Load) process
- MongoDB database
- MongoDB ETL process

The script automatically cleans up old WAL (Write-Ahead Logging) files and uses Overmind to manage all processes. You can press Ctrl+C to stop all services.

### Using the Golem Base CLI

The Golem Base CLI allows you to interact with the system through various commands. The CLI is built using the executable in `cmd/golembase/main.go`.

#### Creating an Account

Before interacting with Golem Base, you need to create an account:

```
go run ./cmd/golembase account create
```

This will:
1. Generate a new private key
2. Save it to your configuration directory at `~/.config/golembase/private.key` (macOS/Linux)
3. Display the generated Ethereum address

If an account already exists, it will show the existing address.

#### Funding an Account

To add funds to your account (necessary for creating entities):

```
go run ./cmd/golembase account fund
```

This command:
1. Loads your account information
2. Connects to the local Golem Base node (by default at http://localhost:8545)
3. Uses an available node account to transfer 100 ETH to your account
4. Waits for the transaction to be mined

Optional flags:
- `--node-url`: Specify a different node URL
- `--value`: Change the amount of ETH to transfer (default: 100)

#### Checking Account Balance

To verify your account balance:

```
go run ./cmd/golembase account balance
```

This displays your account address and current balance in ETH.

#### Creating an Entity

To create a new entity in Golem Base:

```
go run ./cmd/golembase entity create
```

This will:
1. Create an entity with default data ("this is a test") and BTL (100 blocks)
2. Sign and submit a transaction to the node
3. Wait for the transaction to be mined
4. Display the entity key when successful

Optional flags:
- `--node-url`: Specify a different node URL
- `--data`: Custom payload data for the entity
- `--btl`: Custom time-to-live value in blocks

The entity will be stored with:
- Your specified payload
- A default string annotation (key: "foo", value: "bar")
- The entity key derived from the transaction hash, payload, and operation index

Once created, you can query and interact with the entity using the JSON-RPC API methods described earlier.

