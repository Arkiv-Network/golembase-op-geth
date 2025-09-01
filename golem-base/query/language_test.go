package query_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/stretchr/testify/require"
)

func pointerOf[T any](v T) *T {
	return &v
}

func TestParse(t *testing.T) {
	t.Run("quoted string", func(t *testing.T) {

		v, err := query.Parse(`name = "test\"2"`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							Assign: &query.Equality{
								Var: "name",
								Value: &query.Value{
									String: pointerOf("test\"2"),
								},
							},
						},
					},
				},
			},
			v,
		)

	})

	t.Run("number", func(t *testing.T) {
		v, err := query.Parse(`name = 123`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							Assign: &query.Equality{
								Var: "name",
								Value: &query.Value{
									Number: pointerOf(uint64(123)),
								},
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("lessthan", func(t *testing.T) {
		v, err := query.Parse(`name < 123`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							LessThan: &query.LessThan{
								Var: "name",
								Value: &query.Value{
									Number: pointerOf(uint64(123)),
								},
							},
						},
					},
				},
			},
			v,
		)

		v, err = query.Parse(`name < "123"`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							LessThan: &query.LessThan{
								Var: "name",
								Value: &query.Value{
									String: pointerOf("123"),
								},
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("lessthanequal", func(t *testing.T) {
		v, err := query.Parse(`name <= 123`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							LessOrEqualThan: &query.LessOrEqualThan{
								Var: "name",
								Value: &query.Value{
									Number: pointerOf(uint64(123)),
								},
							},
						},
					},
				},
			},
			v,
		)

		v, err = query.Parse(`name <= "123"`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							LessOrEqualThan: &query.LessOrEqualThan{
								Var: "name",
								Value: &query.Value{
									String: pointerOf("123"),
								},
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("greaterthan", func(t *testing.T) {
		v, err := query.Parse(`name > 123`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							GreaterThan: &query.GreaterThan{
								Var: "name",
								Value: &query.Value{
									Number: pointerOf(uint64(123)),
								},
							},
						},
					},
				},
			},
			v,
		)

		v, err = query.Parse(`name > "123"`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							GreaterThan: &query.GreaterThan{
								Var: "name",
								Value: &query.Value{
									String: pointerOf("123"),
								},
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("greaterthanequal", func(t *testing.T) {
		v, err := query.Parse(`name >= 123`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							GreaterOrEqualThan: &query.GreaterOrEqualThan{
								Var: "name",
								Value: &query.Value{
									Number: pointerOf(uint64(123)),
								},
							},
						},
					},
				},
			},
			v,
		)

		v, err = query.Parse(`name >= "123"`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							GreaterOrEqualThan: &query.GreaterOrEqualThan{
								Var: "name",
								Value: &query.Value{
									String: pointerOf("123"),
								},
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("glob", func(t *testing.T) {
		v, err := query.Parse(`name ~ "foo"`)
		require.NoError(t, err)

		require.Equal(
			t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							Glob: &query.Glob{
								Var:   "name",
								Value: "foo",
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("and", func(t *testing.T) {
		v, err := query.Parse(`name = 123 && name2 = "abc"`)
		require.NoError(t, err)

		require.Equal(t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							Assign: &query.Equality{
								Var: "name",
								Value: &query.Value{
									Number: pointerOf(uint64(123)),
								},
							},
						},
						Right: []*query.AndRHS{
							{
								Expr: &query.EqualExpr{
									Assign: &query.Equality{
										Var: "name2",
										Value: &query.Value{
											String: pointerOf("abc"),
										},
									},
								},
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("or", func(t *testing.T) {
		v, err := query.Parse(`name = 123 || name2 = "abc"`)
		require.NoError(t, err)

		require.Equal(t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							Assign: &query.Equality{
								Var: "name",
								Value: &query.Value{
									Number: pointerOf(uint64(123)),
								},
							},
						},
					},
					Right: []*query.OrRHS{
						{
							Expr: &query.AndExpression{
								Left: &query.EqualExpr{
									Assign: &query.Equality{
										Var: "name2",
										Value: &query.Value{
											String: pointerOf("abc"),
										},
									},
								},
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("parentheses", func(t *testing.T) {
		v, err := query.Parse(`(name = 123 || name2 = "abc") && name3 = "def" || name4 = 456`)
		require.NoError(t, err)

		require.Equal(t,
			&query.Expression{
				Or: &query.OrExpression{
					Left: &query.AndExpression{
						Left: &query.EqualExpr{
							Paren: &query.Expression{
								Or: &query.OrExpression{
									Left: &query.AndExpression{
										Left: &query.EqualExpr{
											Assign: &query.Equality{
												Var: "name",
												Value: &query.Value{
													Number: pointerOf(uint64(123)),
												},
											},
										},
									},
									Right: []*query.OrRHS{
										{
											Expr: &query.AndExpression{
												Left: &query.EqualExpr{
													Assign: &query.Equality{
														Var: "name2",
														Value: &query.Value{
															String: pointerOf("abc"),
														},
													},
												},
											},
										},
									},
								},
							},
						},
						Right: []*query.AndRHS{
							{
								Expr: &query.EqualExpr{
									Assign: &query.Equality{
										Var: "name3",
										Value: &query.Value{
											String: pointerOf("def"),
										},
									},
								},
							},
						},
					},
					Right: []*query.OrRHS{
						{
							Expr: &query.AndExpression{
								Left: &query.EqualExpr{
									Assign: &query.Equality{
										Var: "name4",
										Value: &query.Value{
											Number: pointerOf(uint64(456)),
										},
									},
								},
							},
						},
					},
				},
			},
			v,
		)
	})

	t.Run("invalid expression", func(t *testing.T) {
		_, err := query.Parse(`key = 8e`)
		require.Error(t, err, `1:8: unexpected token "e"`)
	})

}
