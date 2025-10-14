package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

// golemBaseAPI offers helper utils
type golemBaseAPI struct {
	eth   *Ethereum
	store *sqlstore.SQLStore
}

func NewGolemBaseAPI(eth *Ethereum, store *sqlstore.SQLStore) *golemBaseAPI {
	return &golemBaseAPI{
		eth:   eth,
		store: store,
	}
}

func (api *golemBaseAPI) GetStorageValue(ctx context.Context, key common.Hash) ([]byte, error) {
	payload, err := api.store.GetQueries().GetEntityPayload(ctx, key.Hex())
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (api *golemBaseAPI) GetEntityMetaData(ctx context.Context, key common.Hash) (*entity.EntityMetaData, error) {
	metadata, err := api.store.GetEntityMetaData(ctx, key)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

func (api *golemBaseAPI) GetEntitiesToExpireAtBlock(ctx context.Context, blockNumber uint64) ([]common.Hash, error) {
	entities, err := api.store.GetEntitiesToExpireAtBlock(ctx, blockNumber)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

func (api *golemBaseAPI) GetEntitiesForStringAnnotationValue(ctx context.Context, key, value string) ([]common.Hash, error) {
	entities, err := api.store.GetEntitiesForStringAnnotationValue(ctx, key, value)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

func (api *golemBaseAPI) GetEntitiesForNumericAnnotationValue(ctx context.Context, key string, value uint64) ([]common.Hash, error) {
	entities, err := api.store.GetEntitiesForNumericAnnotationValue(ctx, key, value)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

func (api *golemBaseAPI) QueryEntities(ctx context.Context, req string) ([]golemtype.SearchResult, error) {

	expr, err := query.Parse(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	query := expr.Evaluate()

	entities, err := api.store.QueryEntities(ctx, query.Query, query.Args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	searchResults := make([]golemtype.SearchResult, 0)

	for _, key := range entities {
		v, err := api.GetStorageValue(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("failed to get storage value for key %s: %w", key.Hex(), err)
		}
		searchResults = append(searchResults, golemtype.SearchResult{
			Key:   key,
			Value: v,
		})
	}

	return searchResults, nil
}

// GetEntityCount returns the total number of entities in the storage.
func (api *golemBaseAPI) GetEntityCount(ctx context.Context) (uint64, error) {
	count, err := api.store.GetEntityCount(ctx)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetAllEntityKeys returns all entity keys in the storage.
func (api *golemBaseAPI) GetAllEntityKeys(ctx context.Context) ([]common.Hash, error) {
	entities, err := api.store.GetAllEntityKeys(ctx)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

func (api *golemBaseAPI) GetEntitiesOfOwner(ctx context.Context, owner common.Address) ([]common.Hash, error) {
	entities, err := api.store.GetEntitiesOfOwner(ctx, owner)
	if err == nil {
		return entities, nil
	}

	return entities, nil
}

func (api *golemBaseAPI) GetNumberOfUsedSlots() (*hexutil.Big, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	counter := storageaccounting.GetNumberOfUsedSlots(stateDb)
	counterAsBigInt := big.NewInt(0)
	counter.IntoBig(&counterAsBigInt)
	return (*hexutil.Big)(counterAsBigInt), nil
}
