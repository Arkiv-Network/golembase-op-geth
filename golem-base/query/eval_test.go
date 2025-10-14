package query_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/stretchr/testify/require"
)

func TestEqualExpr(t *testing.T) {
	expr, err := query.Parse("name = \"test\"")
	require.NoError(t, err)

	res := expr.Evaluate()

	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?)",
			"SELECT entity_key FROM table_1",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"name",
			"test",
		},
		res.Args,
	)

	// Query for a key with special characters
	expr, err = query.Parse("déçevant = \"non\"")
	require.NoError(t, err)

	res = expr.Evaluate()

	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?)",
			"SELECT entity_key FROM table_1",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"déçevant",
			"non",
		},
		res.Args,
	)

	expr, err = query.Parse("بروح = \"ايوة\"")
	require.NoError(t, err)

	res = expr.Evaluate()
	require.NoError(t, err)

	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?)",
			"SELECT entity_key FROM table_1",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
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

	res := expr.Evaluate()
	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?)",
			"SELECT entity_key FROM table_1",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"age",
			uint64(123),
		},
		res.Args,
	)
}

func TestAndExpr(t *testing.T) {
	expr, err := query.Parse(`age = 123 && name = "abc"`)
	require.NoError(t, err)

	res := expr.Evaluate()
	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_2 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_3 AS (SELECT entity_key FROM table_1 INTERSECT SELECT entity_key FROM table_2)",
			"SELECT entity_key FROM table_3",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"age",
			uint64(123),
			"name",
			"abc",
		},
		res.Args,
	)
}

func TestOrExpr(t *testing.T) {
	expr, err := query.Parse(`age = 123 || name = "abc"`)
	require.NoError(t, err)

	res := expr.Evaluate()
	require.NoError(t, err)
	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_2 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_3 AS (SELECT entity_key FROM table_1 UNION SELECT entity_key FROM table_2)",
			"SELECT entity_key FROM table_3",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"age",
			uint64(123),
			"name",
			"abc",
		},
		res.Args,
	)
}

func TestParenthesesExpr(t *testing.T) {
	expr, err := query.Parse(`(name = 123 || name2 = "abc") && name3 = "def" || (name4 = 456 && name5 = 567)`)
	require.NoError(t, err)

	res := expr.Evaluate()
	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_2 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_3 AS (SELECT entity_key FROM table_1 UNION SELECT entity_key FROM table_2),",
			"table_4 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_5 AS (SELECT entity_key FROM table_3 INTERSECT SELECT entity_key FROM table_4),",
			"table_6 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_7 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_8 AS (SELECT entity_key FROM table_6 INTERSECT SELECT entity_key FROM table_7),",
			"table_9 AS (SELECT entity_key FROM table_5 UNION SELECT entity_key FROM table_8)",
			"SELECT entity_key FROM table_9",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"name",
			uint64(123),
			"name2",
			"abc",
			"name3",
			"def",
			"name4",
			uint64(456),
			"name5",
			uint64(567),
		},
		res.Args,
	)
}

func TestOwner(t *testing.T) {
	owner := common.HexToAddress("0x1")

	expr, err := query.Parse(fmt.Sprintf(`(age = 123 || name = "abc") && $owner = "%s"`, owner))
	require.NoError(t, err)

	res := expr.Evaluate()

	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_2 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_3 AS (SELECT entity_key FROM table_1 UNION SELECT entity_key FROM table_2),",
			"table_4 AS (SELECT key AS entity_key FROM entities WHERE owner_address = ?),",
			"table_5 AS (SELECT entity_key FROM table_3 INTERSECT SELECT entity_key FROM table_4)",
			"SELECT entity_key FROM table_5",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"age",
			uint64(123),
			"name",
			"abc",
			owner.Hex(),
		},
		res.Args,
	)
}

func TestGlob(t *testing.T) {
	expr, err := query.Parse(`age ~ "abc"`)
	require.NoError(t, err)

	res := expr.Evaluate()

	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value GLOB ?)",
			"SELECT entity_key FROM table_1",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"age",
			"abc",
		},
		res.Args,
	)
}

func TestNegation(t *testing.T) {
	expr, err := query.Parse(
		`!(name < 123 || !(name2 = "abc" && name2 != "bcd")) && !(name3 = "def") || name4 = 456`,
	)

	require.NoError(t, err)

	res := expr.Evaluate()

	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value >= ?),",
			"table_2 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_3 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value != ?),",
			"table_4 AS (SELECT entity_key FROM table_2 INTERSECT SELECT entity_key FROM table_3),",
			"table_5 AS (SELECT entity_key FROM table_1 INTERSECT SELECT entity_key FROM table_4),",
			"table_6 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value != ?),",
			"table_7 AS (SELECT entity_key FROM table_5 INTERSECT SELECT entity_key FROM table_6),",
			"table_8 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_9 AS (SELECT entity_key FROM table_7 UNION SELECT entity_key FROM table_8)",
			"SELECT entity_key FROM table_9",
			"ORDER BY 1",
		},
			" ",
		),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"name",
			uint64(123),
			"name2",
			"abc",
			"name2",
			"bcd",
			"name3",
			"def",
			"name4",
			uint64(456),
		},
		res.Args,
	)
}

func TestAndExpr_MultipleTerms(t *testing.T) {
	expr, err := query.Parse(`a = 1 && b = "x" && c = 2 && d = "y"`)
	require.NoError(t, err)

	res := expr.Evaluate()
	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_2 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_3 AS (SELECT entity_key FROM table_1 INTERSECT SELECT entity_key FROM table_2),",
			"table_4 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_5 AS (SELECT entity_key FROM table_3 INTERSECT SELECT entity_key FROM table_4),",
			"table_6 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_7 AS (SELECT entity_key FROM table_5 INTERSECT SELECT entity_key FROM table_6)",
			"SELECT entity_key FROM table_7",
			"ORDER BY 1",
		}, " "),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"a", uint64(1),
			"b", "x",
			"c", uint64(2),
			"d", "y",
		},
		res.Args,
	)
}

func TestOrExpr_MultipleTerms(t *testing.T) {
	expr, err := query.Parse(`a = 1 || b = "x" || c = 2 || d = "y"`)
	require.NoError(t, err)

	res := expr.Evaluate()
	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_2 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_3 AS (SELECT entity_key FROM table_1 UNION SELECT entity_key FROM table_2),",
			"table_4 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_5 AS (SELECT entity_key FROM table_3 UNION SELECT entity_key FROM table_4),",
			"table_6 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_7 AS (SELECT entity_key FROM table_5 UNION SELECT entity_key FROM table_6)",
			"SELECT entity_key FROM table_7",
			"ORDER BY 1",
		}, " "),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"a", uint64(1),
			"b", "x",
			"c", uint64(2),
			"d", "y",
		},
		res.Args,
	)
}

func TestMixedAndOr_NoParens(t *testing.T) {
	expr, err := query.Parse(`a = 1 && b = "x" || c = 2 && d = "y"`)
	require.NoError(t, err)

	res := expr.Evaluate()
	require.Equal(t,
		strings.Join([]string{
			"WITH",
			"table_1 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_2 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_3 AS (SELECT entity_key FROM table_1 INTERSECT SELECT entity_key FROM table_2),",
			"table_4 AS (SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?),",
			"table_5 AS (SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?),",
			"table_6 AS (SELECT entity_key FROM table_4 INTERSECT SELECT entity_key FROM table_5),",
			"table_7 AS (SELECT entity_key FROM table_3 UNION SELECT entity_key FROM table_6)",
			"SELECT entity_key FROM table_7",
			"ORDER BY 1",
		}, " "),
		res.Query,
	)

	require.ElementsMatch(t,
		[]any{
			"a", uint64(1),
			"b", "x",
			"c", uint64(2),
			"d", "y",
		},
		res.Args,
	)
}
