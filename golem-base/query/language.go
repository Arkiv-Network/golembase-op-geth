package query

import (
	"errors"
	"fmt"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

// Define the lexer with distinct tokens for each operator and parentheses.
var lex = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Whitespace", Pattern: `[ \t\n\r]+`},
	{Name: "LParen", Pattern: `\(`},
	{Name: "RParen", Pattern: `\)`},
	{Name: "And", Pattern: `&&`},
	{Name: "Or", Pattern: `\|\|`},
	{Name: "Eq", Pattern: `=`},
	{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
	{Name: "Number", Pattern: `[0-9]+`},
	{Name: "Ident", Pattern: entity.AnnotationIdentRegex},
	// Meta-annotations, should start with $
	{Name: "Owner", Pattern: `\$owner`},
})

// Expression is the top-level rule.
type Expression struct {
	Or *OrExpression `parser:"@@"`
}

func (e *Expression) Evaluate(ds DataSource) ([]common.Hash, error) {
	return e.Or.Evaluate(ds)
}

// OrExpression handles expressions connected with ||.
type OrExpression struct {
	Left  *AndExpression `parser:"@@"`
	Right []*OrRHS       `parser:"@@*"`
}

func union(a, b []common.Hash) []common.Hash {
	result := make([]common.Hash, 0, len(a)+len(b))
	seen := make(map[common.Hash]bool)

	// Add all hashes from a
	for _, hash := range a {
		if !seen[hash] {
			seen[hash] = true
			result = append(result, hash)
		}
	}

	// Add any new hashes from b
	for _, hash := range b {
		if !seen[hash] {
			seen[hash] = true
			result = append(result, hash)
		}
	}

	return result
}

func (e *OrExpression) Evaluate(ds DataSource) ([]common.Hash, error) {
	res, err := e.Left.Evaluate(ds)
	if err != nil {
		return nil, err
	}

	for _, rhs := range e.Right {
		rh, err := rhs.Evaluate(ds)
		if err != nil {
			return nil, err
		}
		res = union(res, rh)
	}

	return res, nil
}

// OrRHS represents the right-hand side of an OR.
type OrRHS struct {
	Op   string         `parser:"@Or"`
	Expr *AndExpression `parser:"@@"`
}

func (e *OrRHS) Evaluate(ds DataSource) ([]common.Hash, error) {
	return e.Expr.Evaluate(ds)
}

// AndExpression handles expressions connected with &&.
type AndExpression struct {
	Left  *EqualExpr `parser:"@@"`
	Right []*AndRHS  `parser:"@@*"`
}

func intersect(a, b []common.Hash) []common.Hash {
	result := make([]common.Hash, 0)
	seen := make(map[common.Hash]bool)

	// Build map of hashes in a
	for _, hash := range a {
		seen[hash] = true
	}

	// Check which hashes from b exist in map
	for _, hash := range b {
		if seen[hash] {
			result = append(result, hash)
		}
	}

	return result

}

func (e *AndExpression) Evaluate(ds DataSource) ([]common.Hash, error) {

	res, err := e.Left.Evaluate(ds)
	if err != nil {
		return nil, err
	}

	for _, rhs := range e.Right {
		rh, err := rhs.Evaluate(ds)
		if err != nil {
			return nil, err
		}
		res = intersect(res, rh)
	}

	return res, nil
}

// AndRHS represents the right-hand side of an AND.
type AndRHS struct {
	Op   string     `parser:"@And"`
	Expr *EqualExpr `parser:"@@"`
}

func (e *AndRHS) Evaluate(ds DataSource) ([]common.Hash, error) {
	return e.Expr.Evaluate(ds)
}

// EqualExpr can be either an equality or a parenthesized expression.
type EqualExpr struct {
	Paren  *Expression `parser:"  \"(\" @@ \")\""`
	Owner  *Ownership  `parser:"| @@"`
	Assign *Equality   `parser:"| @@"`
}

func (e *EqualExpr) Evaluate(ds DataSource) ([]common.Hash, error) {
	if e.Paren != nil {
		return e.Paren.Evaluate(ds)
	}

	if e.Owner != nil {
		return e.Owner.Evaluate(ds)
	}

	return e.Assign.Evaluate(ds)
}

// Ownership represents an ownership query, $owner = 0x....
type Ownership struct {
	Owner *string `parser:"Owner '=' @String"`
}

func (e *Ownership) Evaluate(ds DataSource) ([]common.Hash, error) {

	if common.IsHexAddress(*e.Owner) {
		address := common.HexToAddress(*e.Owner)
		return ds.GetKeysForOwner(address)
	}

	return nil, fmt.Errorf(
		"invalid value for owner, expected 20-byte hex string, got: %s",
		*e.Owner,
	)

}

// Equality represents a simple equality (e.g. name = 123).
type Equality struct {
	Var   string `parser:"@Ident \"=\""`
	Value *Value `parser:"@@"`
}

func (e *Equality) Evaluate(ds DataSource) ([]common.Hash, error) {

	if e.Value.String != nil {
		return ds.GetKeysForStringAnnotation(e.Var, *e.Value.String)
	}

	if e.Value.Number != nil {
		return ds.GetKeysForNumericAnnotation(e.Var, *e.Value.Number)
	}

	return nil, errors.New("unsupported value type")
}

// Value is a literal value (a number or a string).
type Value struct {
	String *string `parser:"  @String"`
	Number *uint64 `parser:"| @Number"`
}

var Parser = participle.MustBuild[Expression](
	participle.Lexer(lex),
	participle.Elide("Whitespace"),
	participle.Unquote("String"),
)

func Parse(s string) (*Expression, error) {
	v, err := Parser.ParseString("", s)
	return v, err
}
