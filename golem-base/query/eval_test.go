package query_test

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/stretchr/testify/require"
)

var queryOptions = query.QueryOptions{}

func TestEqualExpr(t *testing.T) {
	expr, err := query.Parse("name = \"test\"")
	require.NoError(t, err)

	res := expr.Evaluate(queryOptions)

	block := uint64(0)

	require.ElementsMatch(t,
		[]any{
			block, block,
			"name",
			"test",
		},
		res.Args,
	)

	// Query for a key with special characters
	expr, err = query.Parse("déçevant = \"non\"")
	require.NoError(t, err)

	res = expr.Evaluate(queryOptions)

	require.ElementsMatch(t,
		[]any{
			block, block,
			"déçevant",
			"non",
		},
		res.Args,
	)

	expr, err = query.Parse("بروح = \"ايوة\"")
	require.NoError(t, err)

	res = expr.Evaluate(queryOptions)
	require.NoError(t, err)

	require.ElementsMatch(t,
		[]any{
			block, block,
			"بروح",
			"ايوة",
		},
		res.Args,
	)

	// But symbols should fail
	_, err = query.Parse("foo@ = \"bar\"")
	require.Error(t, err)
}

func TestNumericEqualExpr(t *testing.T) {
	expr, err := query.Parse("age = 123")
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestAndExpr(t *testing.T) {
	expr, err := query.Parse(`age = 123 && name = "abc"`)
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestOrExpr(t *testing.T) {
	expr, err := query.Parse(`age = 123 || name = "abc"`)
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestParenthesesExpr(t *testing.T) {
	expr, err := query.Parse(`(name = 123 || name2 = "abc") && name3 = "def" || (name4 = 456 && name5 = 567)`)
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestOwner(t *testing.T) {
	owner := common.HexToAddress("0x1")

	expr, err := query.Parse(fmt.Sprintf(`(age = 123 || name = "abc") && $owner = %s`, owner))
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestGlob(t *testing.T) {
	expr, err := query.Parse(`age ~ "abc"`)
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestNegation(t *testing.T) {
	expr, err := query.Parse(
		`!(name < 123 || !(name2 = "abc" && name2 != "bcd")) && !(name3 = "def") || name4 = 456`,
	)

	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestAndExpr_MultipleTerms(t *testing.T) {
	expr, err := query.Parse(`a = 1 && b = "x" && c = 2 && d = "y"`)
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestOrExpr_MultipleTerms(t *testing.T) {
	expr, err := query.Parse(`a = 1 || b = "x" || c = 2 || d = "y"`)
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}

func TestMixedAndOr_NoParens(t *testing.T) {
	expr, err := query.Parse(`a = 1 && b = "x" || c = 2 && d = "y"`)
	require.NoError(t, err)

	expr.Evaluate(queryOptions)
}
