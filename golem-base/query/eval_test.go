package query_test

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/stretchr/testify/require"
)

// var _ query.Evaluator = &query.EqualExpr{}

type fakeDataSource struct {
	stringAnnotations  map[string]map[string][]common.Hash
	numericAnnotations map[string]map[uint64][]common.Hash
	ownerAddresses     map[common.Address][]common.Hash
}

func (f *fakeDataSource) GetKeysForStringAnnotation(key, value string) ([]common.Hash, error) {
	return f.stringAnnotations[key][value], nil
}

func (f *fakeDataSource) GetKeysForNumericAnnotation(key string, value uint64) ([]common.Hash, error) {
	return f.numericAnnotations[key][value], nil
}

func (f *fakeDataSource) GetKeysForOwner(owner common.Address) ([]common.Hash, error) {
	return f.ownerAddresses[owner], nil
}

func TestEqualExpr(t *testing.T) {
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name": {
				"test":  []common.Hash{common.HexToHash("0x1")},
				"test2": []common.Hash{common.HexToHash("0x2")},
			},
			"déçevant": {
				"non": []common.Hash{common.HexToHash("0x3")},
			},
			"بروح": {
				"ايوة": []common.Hash{common.HexToHash("0x3")},
			},
		},
		numericAnnotations: map[string]map[uint64][]common.Hash{},
	}

	expr, err := query.Parse("name = \"test\"")
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)

	require.Equal(t, []common.Hash{common.HexToHash("0x1")}, res)

	// Query for a key with special characters
	expr, err = query.Parse("déçevant = \"non\"")
	require.NoError(t, err)

	res, err = expr.Evaluate(ds)
	require.NoError(t, err)

	require.Equal(t, []common.Hash{common.HexToHash("0x3")}, res)

	expr, err = query.Parse("بروح = \"ايوة\"")
	require.NoError(t, err)

	res, err = expr.Evaluate(ds)
	require.NoError(t, err)

	require.Equal(t, []common.Hash{common.HexToHash("0x3")}, res)

	// But symbols should fail
	expr, err = query.Parse("foo@ = \"bar\"")
	require.Error(t, err)
}

func TestNumericEqualExpr(t *testing.T) {
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{},
		numericAnnotations: map[string]map[uint64][]common.Hash{
			"age": {
				123: []common.Hash{common.HexToHash("0x1")},
				456: []common.Hash{common.HexToHash("0x2")},
			},
		},
	}

	expr, err := query.Parse("age = 123")
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.Equal(t, []common.Hash{common.HexToHash("0x1")}, res)
}

func TestAndExpr(t *testing.T) {
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name": {
				"abc": []common.Hash{common.HexToHash("0x1"), common.HexToHash("0x3")},
			},
		},
		numericAnnotations: map[string]map[uint64][]common.Hash{
			"age": {
				123: []common.Hash{common.HexToHash("0x1"), common.HexToHash("0x2")},
			},
		},
	}

	expr, err := query.Parse(`age = 123 && name = "abc"`)
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.Equal(t, []common.Hash{common.HexToHash("0x1")}, res)
}

func TestOrExpr(t *testing.T) {
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name": {
				"abc": []common.Hash{common.HexToHash("0x3")},
			},
		},
		numericAnnotations: map[string]map[uint64][]common.Hash{
			"age": {
				123: []common.Hash{common.HexToHash("0x1"), common.HexToHash("0x2")},
			},
		},
	}

	expr, err := query.Parse(`age = 123 || name = "abc"`)
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.ElementsMatch(t, []common.Hash{
		common.HexToHash("0x1"),
		common.HexToHash("0x2"),
		common.HexToHash("0x3"),
	}, res)
}

func TestParenthesesExpr(t *testing.T) {
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name2": {
				"abc": []common.Hash{common.HexToHash("0x2"), common.HexToHash("0x3")},
			},
			"name3": {
				"def": []common.Hash{common.HexToHash("0x3"), common.HexToHash("0x4")},
			},
		},
		numericAnnotations: map[string]map[uint64][]common.Hash{
			"name": {
				123: []common.Hash{common.HexToHash("0x1"), common.HexToHash("0x2")},
			},
			"name4": {
				456: []common.Hash{common.HexToHash("0x5")},
			},
		},
	}

	expr, err := query.Parse(`(name = 123 || name2 = "abc") && name3 = "def" || name4 = 456`)
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.ElementsMatch(t, []common.Hash{
		common.HexToHash("0x3"),
		common.HexToHash("0x5"),
	}, res)
}

func TestOwner(t *testing.T) {
	owner := common.HexToAddress("0x1")

	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name": {
				"abc": []common.Hash{common.HexToHash("0x3")},
			},
		},
		numericAnnotations: map[string]map[uint64][]common.Hash{
			"age": {
				123: []common.Hash{common.HexToHash("0x1"), common.HexToHash("0x2")},
			},
		},
		ownerAddresses: map[common.Address][]common.Hash{
			owner: {common.HexToHash("0x1"), common.HexToHash("0x3")},
		},
	}

	expr, err := query.Parse(fmt.Sprintf(`(age = 123 || name = "abc") && $owner = "%s"`, owner))
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.ElementsMatch(t, []common.Hash{
		common.HexToHash("0x1"),
		common.HexToHash("0x3"),
	}, res)
}
