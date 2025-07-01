package eth

import (
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/annotationindex"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entitiesofowner"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
)

// golemBaseAPI offers helper utils
type golemBaseAPI struct {
	eth *Ethereum
}

func NewGolemBaseAPI(eth *Ethereum) *golemBaseAPI {
	return &golemBaseAPI{
		eth: eth,
	}
}

func (api *golemBaseAPI) GetStorageValue(key common.Hash) ([]byte, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	return entity.GetPayload(stateDb, key), nil
}

func (api *golemBaseAPI) GetEntityMetaData(key common.Hash) (*entity.EntityMetaData, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	return entity.GetEntityMetaData(stateDb, key)
}

func (api *golemBaseAPI) GetEntitiesToExpireAtBlock(blockNumber uint64) ([]common.Hash, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	out := slices.Collect(entityexpiration.IteratorOfEntitiesToExpireAtBlock(stateDb, blockNumber))
	if out == nil {
		out = make([]common.Hash, 0)
	}
	return out, nil
}

func (api *golemBaseAPI) GetEntitiesForStringAnnotationValue(key, value string) ([]common.Hash, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	entitySetKey := annotationindex.StringAnnotationIndexKey(key, value)

	out := slices.Collect(keyset.Iterate(stateDb, entitySetKey))
	if out == nil {
		out = make([]common.Hash, 0)
	}
	return out, nil
}

func (api *golemBaseAPI) GetEntitiesForNumericAnnotationValue(key string, value uint64) ([]common.Hash, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	entityKeys := annotationindex.NumericAnnotationIndexKey(key, value)

	out := slices.Collect(keyset.Iterate(stateDb, entityKeys))
	if out == nil {
		out = make([]common.Hash, 0)
	}
	return out, nil
}

func (api *golemBaseAPI) QueryEntities(req string) ([]golemtype.SearchResult, error) {

	expr, err := query.Parse(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	ds := &golemBaseDataSource{api: api}
	entities, err := expr.Evaluate(ds)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate query: %w", err)
	}

	searchResults := make([]golemtype.SearchResult, 0)

	for _, key := range entities {
		v, err := api.GetStorageValue(key)
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

type golemBaseDataSource struct {
	api *golemBaseAPI
}

func (ds *golemBaseDataSource) GetKeysForStringAnnotation(key, value string) ([]common.Hash, error) {
	return ds.api.GetEntitiesForStringAnnotationValue(key, value)
}

func (ds *golemBaseDataSource) GetKeysForNumericAnnotation(key string, value uint64) ([]common.Hash, error) {
	return ds.api.GetEntitiesForNumericAnnotationValue(key, value)
}

func (ds *golemBaseDataSource) GetKeysForOwner(owner common.Address) ([]common.Hash, error) {
	return ds.api.GetEntitiesOfOwner(owner)
}

// GetEntityCount returns the total number of entities in the storage.
func (api *golemBaseAPI) GetEntityCount() (uint64, error) {
	stateDb, err := api.eth.BlockChain().StateAt(api.eth.BlockChain().CurrentHeader().Root)
	if err != nil {
		return 0, fmt.Errorf("failed to get state: %w", err)
	}

	// Use keyset.Size to get the count of entities from the global registry
	count := keyset.Size(stateDb, allentities.AllEntitiesKey)

	return count.Uint64(), nil
}

// GetAllEntityKeys returns all entity keys in the storage.
func (api *golemBaseAPI) GetAllEntityKeys() ([]common.Hash, error) {
	stateDb, err := api.eth.BlockChain().StateAt(api.eth.BlockChain().CurrentHeader().Root)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	// Use the iterator from allentities package to gather all entity hashes
	var entityKeys []common.Hash

	for hash := range allentities.Iterate(stateDb) {
		entityKeys = append(entityKeys, hash)
	}

	if entityKeys == nil {
		entityKeys = make([]common.Hash, 0)
	}
	return entityKeys, nil
}

func (api *golemBaseAPI) GetEntitiesOfOwner(owner common.Address) ([]common.Hash, error) {
	stateDb, err := api.eth.BlockChain().StateAt(api.eth.BlockChain().CurrentHeader().Root)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	entityKeys := slices.Collect(entitiesofowner.Iterate(stateDb, owner))

	if entityKeys == nil {
		entityKeys = make([]common.Hash, 0)
	}
	return entityKeys, nil
}
