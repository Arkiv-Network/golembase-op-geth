# Golem Base CLI

The Golem Base CLI is a command-line interface for interacting with the Golem Base storage system (golembase-op-geth). It provides tools for account management, entity creation, and querying the storage system.

## Configuration

The CLI follows the XDG Base Directory Specification for storing configuration files:
- On macOS: `~/Library/Application Support/golembase/`
- On Linux: `~/.config/golembase/`
- On Windows: `%LOCALAPPDATA%\golembase\`

## Available Commands

### Account Management

- `account create`: Creates a new account
  - Generates a new `wallet.json` file
  - Saves it to the XDG config directory (e.g., `~/Library/Application Support/golembase/wallet.json` on macOS)
  - Displays the generated Ethereum address

- `account fund`: Funds an account with ETH
  - Connects to local Golem Base node (default: http://localhost:8545)
  - Transfers 100 ETH to your account (works only with unlocked accounts, e.g. in dev mode)
  - Optional flags:
    - `--node-url`: Specify different node URL
    - `--value`: Change amount of ETH to transfer

- `account balance`: Checks account balance
  - Displays account address and current ETH balance

### Entity Management

- `entity create`: Creates a new entity in Golem Base
  - Creates entity with default data and BTL (100 blocks)
  - Signs and submits transaction to the node
  - Optional flags:
    - `--node-url`: Specify different node URL
    - `--data`: Custom payload data
    - `--btl`: Custom time-to-live value in blocks

### Query Operations

- `query`: Commands for querying the storage system
  - Execute custom queries using the Golem Base query language
  - Search entities by annotations
  - Retrieve entity metadata
  - For detailed query syntax and examples, see the [Query Language Support section](../../golem-base/README.md#query-language-support)

### Entity Content Display

- `cat`: Display entity payload content
  - Similar to Unix `cat` command
  - Dumps the raw payload data of a specified entity
  - Useful for viewing the contents of stored entities

## Usage Examples

1. Create a new account:
```bash
golembase account create
```

2. Fund your account:
```bash
golembase account fund
```

3. Create a new entity:
```bash
golembase entity create --data "custom data" --btl 200
```

4. Display entity payload:
```bash
golembase cat <entity-key>
```

For more detailed information about the Golem Base system, refer to the main [README.md](../../golem-base/README.md). 
