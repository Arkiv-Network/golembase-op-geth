package eth

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
	"github.com/ethereum/go-ethereum/log"
)

type IncludeData struct {
	Key         bool `json:"key"`
	Annotations bool `json:"annotations"`
	Payload     bool `json:"payload"`
	ContentType bool `json:"contentType"`
	Expiration  bool `json:"expiration"`
	Owner       bool `json:"owner"`
}

type QueryOptions struct {
	AtBlock        *uint64      `json:"atBlock"`
	IncludeData    *IncludeData `json:"includeData"`
	ResultsPerPage uint64       `json:"resultsPerPage"`
	Cursor         *string      `json:"cursor"`
}

var allColumns = []string{
	arkivtype.GetColumnOrPanic("key"),
	arkivtype.GetColumnOrPanic("expires_at"),
	arkivtype.GetColumnOrPanic("owner_address"),
	arkivtype.GetColumnOrPanic("payload"),
	arkivtype.GetColumnOrPanic("content_type"),
}

func verifyColumn(column string) (string, error) {
	verified, err := arkivtype.GetColumn(column)
	if err != nil {
		return "", fmt.Errorf("invalid column supplied in query: %s", column)
	}
	return verified, nil
}

func (options *QueryOptions) getQueryOffset() (*arkivtype.Offset, error) {
	if options.Cursor == nil {
		return nil, nil
	}

	offset := arkivtype.Offset{}
	err := offset.Decode(*options.Cursor)
	if err != nil {
		return nil, err
	}

	for _, val := range offset.ColumnValues {
		_, err := verifyColumn(val.ColumnName)
		if err != nil {
			return nil, err
		}
	}

	return &offset, nil
}

func (options *QueryOptions) toInternalQueryOptions() (*internalQueryOptions, error) {
	switch {
	case options == nil:
		return &internalQueryOptions{
			Columns:            allColumns,
			IncludeAnnotations: true,
		}, nil
	case options.IncludeData == nil:
		offset, err := options.getQueryOffset()
		if err != nil {
			return nil, err
		}
		return &internalQueryOptions{
			Columns:            allColumns,
			IncludeAnnotations: true,
			AtBlock:            options.AtBlock,
			Offset:             offset,
		}, nil
	default:
		offset, err := options.getQueryOffset()
		if err != nil {
			return nil, err
		}
		iq := internalQueryOptions{
			Columns: []string{},
			AtBlock: options.AtBlock,
			Offset:  offset,
		}
		if options.IncludeData.Annotations {
			iq.IncludeAnnotations = true
		}
		if options.IncludeData.Payload {
			iq.Columns = append(iq.Columns, arkivtype.GetColumnOrPanic("payload"))
		}
		if options.IncludeData.ContentType {
			iq.Columns = append(iq.Columns, arkivtype.GetColumnOrPanic("content_type"))
		}
		if options.IncludeData.Expiration {
			iq.Columns = append(iq.Columns, arkivtype.GetColumnOrPanic("expires_at"))
		}
		if options.IncludeData.Owner {
			iq.Columns = append(iq.Columns, arkivtype.GetColumnOrPanic("owner_address"))
		}
		if options.IncludeData.Key {
			iq.Columns = append(iq.Columns, arkivtype.GetColumnOrPanic("key"))
		}
		return &iq, nil
	}
}

type internalQueryOptions struct {
	AtBlock            *uint64           `json:"atBlock"`
	IncludeAnnotations bool              `json:"includeAnnotations"`
	Columns            []string          `json:"columns"`
	Offset             *arkivtype.Offset `json:"offset"`
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

	options, err := op.toInternalQueryOptions()
	if err != nil {
		return nil, err
	}

	latestsHead := api.eth.blockchain.CurrentBlock()

	block := latestsHead.Number.Uint64()
	if options.Offset != nil {
		block = options.Offset.BlockNumber
	}
	if options.AtBlock != nil {
		block = *options.AtBlock
	}

	columnOffsets := []arkivtype.OffsetValue{}
	if options.Offset != nil {
		columnOffsets = options.Offset.ColumnValues
	}

	queryOptions := query.QueryOptions{
		AtBlock:            block,
		IncludeAnnotations: options.IncludeAnnotations,
		Columns:            options.Columns,
		Offset:             columnOffsets,
	}
	query := expr.Evaluate(queryOptions)

	response := &arkivtype.QueryResponse{
		BlockNumber: block,
		Data:        make([]json.RawMessage, 0),
	}

	// In case the query should be run on a block that we don't have yet,
	// we wait for a little bit to see if we receive the block.
	if block > latestsHead.Number.Uint64() {
		var cadence time.Duration
		if latestsHead.Number.Uint64() <= 1 {
			// For block 1, we cannot deduce the cadence, so we just assume 2 seconds
			cadence = 2 * time.Second
		} else {
			cadence = time.Duration(
				latestsHead.Time-api.eth.blockchain.GetHeaderByHash(latestsHead.ParentHash).Time,
			) * time.Second
		}

		time.Sleep(2 * time.Duration(cadence) * time.Second)
		latestsHead = api.eth.blockchain.CurrentBlock()
		if block > latestsHead.Number.Uint64() {
			return nil, fmt.Errorf("requested block is in the future")
		}
	}

	// 256 bytes is for the overhead of the 'envelope' around the entity data
	// and the separator characters in between
	responseSize := 256

	// 512 kb
	maxResponseSize := 512 * 1024 * 1024
	maxResultsPerPage := 0

	if op != nil {
		maxResultsPerPage = int(op.ResultsPerPage)
	}

	startTime := time.Now()

	defer func() {
		elapsed := time.Since(startTime)
		log.Info("query execution time", "elapsed", elapsed)
	}()

	err = api.store.QueryEntitiesInternalIterator(
		ctx,
		query.Query,
		query.Args,
		queryOptions,
		func(entity arkivtype.EntityData, offset arkivtype.Offset) error {

			ed, err := json.Marshal(entity)
			if err != nil {
				return fmt.Errorf("failed to marshal entity: %w", err)
			}

			newLen := responseSize + len(ed) + 1
			if newLen > maxResponseSize {
				cursor, err := offset.Encode()
				if err != nil {
					return fmt.Errorf("could not encode offset: %w", err)
				}
				response.Cursor = &cursor
				return sqlstore.ErrStopIteration
			}
			response.Data = append(response.Data, ed)

			if maxResultsPerPage > 0 && len(response.Data) >= maxResultsPerPage {
				cursor, err := offset.Encode()
				if err != nil {
					return fmt.Errorf("could not encode offset: %w", err)
				}
				response.Cursor = &cursor
				return sqlstore.ErrStopIteration
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return response, nil
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
