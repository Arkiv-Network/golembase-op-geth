package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
)

type IncludeData struct {
	Key         bool `json:"key"`
	Annotations bool `json:"annotations"`
	Payload     bool `json:"payload"`
	Expiration  bool `json:"expiration"`
	Owner       bool `json:"owner"`
}

type QueryOptions struct {
	AtBlock     *uint64      `json:"at_block"`
	IncludeData *IncludeData `json:"include_data"`
}

var allColumns = []string{"key", "expires_at", "owner_address", "payload"}

func (options *QueryOptions) toInternalQueryOptions() internalQueryOptions {

	if options == nil {

	}
	switch {
	case options == nil:
		return internalQueryOptions{
			Columns:            allColumns,
			IncludeAnnotations: true,
		}
	case options.IncludeData == nil:
		return internalQueryOptions{
			Columns:            allColumns,
			IncludeAnnotations: true,
			AtBlock:            options.AtBlock,
		}
	default:
		iq := internalQueryOptions{
			Columns: []string{},
			AtBlock: options.AtBlock,
		}
		if options.IncludeData.Annotations {
			iq.IncludeAnnotations = true
		}
		if options.IncludeData.Payload {
			iq.Columns = append(iq.Columns, "payload")
		}
		if options.IncludeData.Expiration {
			iq.Columns = append(iq.Columns, "expires_at")
		}
		if options.IncludeData.Owner {
			iq.Columns = append(iq.Columns, "owner_address")
		}
		if options.IncludeData.Key {
			iq.Columns = append(iq.Columns, "key")
		}
		return iq
	}
}

type internalQueryOptions struct {
	AtBlock *uint64 `json:"at_block"`
	// TODO Ramses: implement this
	// After that we can use it in GetEntityMetaData
	IncludeAnnotations bool     `json:"include_annotations"`
	Columns            []string `json:"columns"`
}

type arkivAPI struct {
	eth   *Ethereum
	store *sqlstore.SQLStore
}

func NewArkivAPI(eth *Ethereum, store *sqlstore.SQLStore) *arkivAPI {
	return &arkivAPI{
		eth:   eth,
		store: store,
	}
}

func (api *arkivAPI) Query(
	ctx context.Context,
	req string,
	op *QueryOptions,
) (*arkivtype.QueryResponse, error) {

	expr, err := query.Parse(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	options := op.toInternalQueryOptions()

	block := api.eth.blockchain.CurrentBlock().Number.Uint64()
	if options.AtBlock != nil {
		block = *options.AtBlock
	}
	columns := query.COLUMNS
	if len(options.Columns) > 0 {
		columns = options.Columns
	}

	queryOptions := query.QueryOptions{
		AtBlock:            block,
		IncludeAnnotations: options.IncludeAnnotations,
		Columns:            columns,
	}
	query := expr.Evaluate(queryOptions)

	results, err := api.store.QueryEntities(
		ctx,
		query.Query,
		query.Args,
		queryOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return &arkivtype.QueryResponse{
		Data: results,
	}, nil
}

// GetEntityCount returns the total number of entities in the storage.
func (api *arkivAPI) GetEntityCount(ctx context.Context) (uint64, error) {
	count, err := api.store.GetEntityCount(ctx, api.eth.blockchain.CurrentBlock().Number.Uint64())
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (api *arkivAPI) GetNumberOfUsedSlots() (*hexutil.Big, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDB, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	counter := storageaccounting.GetNumberOfUsedSlots(stateDB)
	counterAsBigInt := big.NewInt(0)
	counter.IntoBig(&counterAsBigInt)
	return (*hexutil.Big)(counterAsBigInt), nil
}

type BlockTiming struct {
	CurrentBlock     uint64 `json:"current_block"`
	CurrentBlockTime uint64 `json:"current_block_time"`
	BlockDuration    uint64 `json:"duration"`
}

func (api *arkivAPI) GetBlockTiming(ctx context.Context) (*BlockTiming, error) {
	header := api.eth.blockchain.CurrentHeader()
	previousHeader := api.eth.blockchain.GetHeaderByHash(header.ParentHash)
	if previousHeader == nil {
		return nil, fmt.Errorf("failed to get previous header")
	}

	return &BlockTiming{
		CurrentBlock:     header.Number.Uint64(),
		CurrentBlockTime: header.Time,
		BlockDuration:    header.Time - previousHeader.Time,
	}, nil
}
