package eth

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

// golemBaseAPI offers helper utils
type golemBaseAPI struct {
	*arkivAPI
}

func NewGolemBaseAPI(eth *Ethereum, store *sqlstore.SQLStore) *golemBaseAPI {
	return &golemBaseAPI{
		arkivAPI: NewArkivAPI(eth, store),
	}
}

func (api *golemBaseAPI) GetStorageValue(ctx context.Context, key common.Hash) ([]byte, error) {
	q := fmt.Sprintf(`$key = %s`, key)

	entities, err := api.arkivAPI.Query(
		ctx,
		q,
		&QueryOptions{
			IncludeData: &IncludeData{
				Payload: true,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(entities.Data) != 1 {
		return nil, fmt.Errorf("expected a single result but got %d", len(entities.Data))
	}

	return []byte(entities.Data[0].Value), nil
}

// GetAllEntityKeys returns all entity keys in the storage.
func (api *golemBaseAPI) GetAllEntityKeys(ctx context.Context) ([]common.Hash, error) {
	entities, err := api.store.GetAllEntityKeys(ctx, api.eth.blockchain.CurrentBlock().Number.Uint64())
	if err != nil {
		return nil, err
	}

	return entities, nil
}

func (api *golemBaseAPI) GetEntityMetaData(ctx context.Context, key common.Hash) (*entity.EntityMetaData, error) {
	rows, err := api.arkivAPI.Query(
		ctx,
		fmt.Sprintf("$key = %s", key),
		&QueryOptions{
			IncludeData: &IncludeData{
				Annotations: true,
				Key:         true,
				Expiration:  true,
				Owner:       true,
				Payload:     true,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	if len(rows.Data) != 1 {
		return nil, fmt.Errorf("expected a single result row but got %d", len(rows.Data))
	}

	metadata := rows.Data[0]

	return &entity.EntityMetaData{
		ExpiresAtBlock:     metadata.ExpiresAt,
		Owner:              metadata.Owner,
		StringAnnotations:  metadata.StringAnnotations,
		NumericAnnotations: metadata.NumericAnnotations,
	}, nil
}

func (api *golemBaseAPI) GetEntitiesToExpireAtBlock(ctx context.Context, expirationBlock uint64) ([]common.Hash, error) {
	q := fmt.Sprintf(`$expiration = %d`, expirationBlock)
	entities, err := api.arkivAPI.Query(ctx, q, &QueryOptions{
		IncludeData: &IncludeData{
			Key: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	results := make([]common.Hash, 0, len(entities.Data))
	for _, entity := range entities.Data {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) GetEntitiesForStringAnnotationValue(ctx context.Context, key, value string) ([]common.Hash, error) {
	q := fmt.Sprintf(`%s = "%s"`, key, value)
	entities, err := api.arkivAPI.Query(ctx, q, &QueryOptions{
		IncludeData: &IncludeData{
			Key: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	results := make([]common.Hash, 0, len(entities.Data))
	for _, entity := range entities.Data {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) GetEntitiesForNumericAnnotationValue(ctx context.Context, key string, value uint64) ([]common.Hash, error) {
	q := fmt.Sprintf(`%s = %d`, key, value)
	entities, err := api.arkivAPI.Query(ctx, q, &QueryOptions{
		IncludeData: &IncludeData{
			Key: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	results := make([]common.Hash, 0, len(entities.Data))
	for _, entity := range entities.Data {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) QueryEntities(ctx context.Context, req string) ([]golemtype.SearchResult, error) {
	entities, err := api.arkivAPI.Query(ctx, req, &QueryOptions{
		IncludeData: &IncludeData{
			Key:     true,
			Payload: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	searchResults := make([]golemtype.SearchResult, 0)

	for _, entity := range entities.Data {
		searchResults = append(searchResults, golemtype.SearchResult{
			Key:   entity.Key,
			Value: []byte(entity.Value),
		})
	}

	api.GetEntityCount(ctx)

	return searchResults, nil
}

func (api *golemBaseAPI) GetEntitiesOfOwner(ctx context.Context, owner common.Address) ([]common.Hash, error) {
	q := fmt.Sprintf(`$owner = %s`, owner)
	entities, err := api.arkivAPI.Query(ctx, q, &QueryOptions{
		IncludeData: &IncludeData{
			Key: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	results := make([]common.Hash, 0, len(entities.Data))
	for _, entity := range entities.Data {
		results = append(results, entity.Key)
	}

	return results, nil
}
