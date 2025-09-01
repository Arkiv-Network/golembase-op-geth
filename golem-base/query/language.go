package query

import (
	"fmt"
	"strings"

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
	{Name: "Geqt", Pattern: `>=`},
	{Name: "Leqt", Pattern: `<=`},
	{Name: "Gt", Pattern: `>`},
	{Name: "Lt", Pattern: `<`},
	{Name: "Glob", Pattern: `~`},
	{Name: "Not", Pattern: `!`},
	{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
	{Name: "Number", Pattern: `[0-9]+`},
	{Name: "Ident", Pattern: entity.AnnotationIdentRegex},
	// Meta-annotations, should start with $
	{Name: "Owner", Pattern: `\$owner`},
})

type SelectQuery struct {
	Query string
	Args  []any
}

type QueryBuilder struct {
	tableBuilder *strings.Builder
	args         []any
	needsComma   bool
	tableCounter uint64
}

func (b *QueryBuilder) nextTableName() string {
	b.tableCounter = b.tableCounter + 1
	return fmt.Sprintf("table_%d", b.tableCounter)
}

func (b *QueryBuilder) writeComma() {
	if b.needsComma {
		b.tableBuilder.WriteString(", ")
	} else {
		b.needsComma = true
	}
}

func (b *QueryBuilder) createLeafQuery(query string, args ...any) string {
	tableName := b.nextTableName()
	b.writeComma()
	b.tableBuilder.WriteString(tableName)
	b.tableBuilder.WriteString(" AS (")
	b.tableBuilder.WriteString(query)
	b.tableBuilder.WriteString(")")

	b.args = append(b.args, args...)

	return tableName
}

// Expression is the top-level rule.
type Expression struct {
	Or *OrExpression `parser:"@@"`
}

func (e *Expression) Evaluate() *SelectQuery {
	tableBuilder := strings.Builder{}
	args := []any{}

	tableBuilder.WriteString("WITH ")

	builder := QueryBuilder{
		tableBuilder: &tableBuilder,
		args:         args,
		needsComma:   false,
	}

	tableName := e.Or.Evaluate(&builder)

	tableBuilder.WriteString(" SELECT * FROM ")
	tableBuilder.WriteString(tableName)
	tableBuilder.WriteString(" ORDER BY 1")

	return &SelectQuery{
		Query: tableBuilder.String(),
		Args:  builder.args,
	}
}

func (e *Expression) Recurse(b *QueryBuilder) string {
	// We don't have to do anything here, the parsing order is already taking care
	// of precedence since the nested OR node will create a subquery
	return e.Or.Evaluate(b)
}

// OrExpression handles expressions connected with ||.
type OrExpression struct {
	Left  *AndExpression `parser:"@@"`
	Right []*OrRHS       `parser:"@@*"`
}

func (e *OrExpression) Evaluate(b *QueryBuilder) string {
	leftTable := e.Left.Evaluate(b)
	tableName := leftTable

	for _, rhs := range e.Right {
		rightTable := rhs.Evaluate(b)
		tableName = b.nextTableName()

		b.writeComma()

		b.tableBuilder.WriteString(tableName)
		b.tableBuilder.WriteString(" AS (")
		b.tableBuilder.WriteString("SELECT * FROM ")
		b.tableBuilder.WriteString(leftTable)
		b.tableBuilder.WriteString(" UNION ")
		b.tableBuilder.WriteString("SELECT * FROM ")
		b.tableBuilder.WriteString(rightTable)
		b.tableBuilder.WriteString(")")

		leftTable = rightTable
	}

	return tableName
}

// OrRHS represents the right-hand side of an OR.
type OrRHS struct {
	Expr *AndExpression `parser:"Or @@"`
}

func (e *OrRHS) Evaluate(b *QueryBuilder) string {
	return e.Expr.Evaluate(b)
}

// AndExpression handles expressions connected with &&.
type AndExpression struct {
	Left  *EqualExpr `parser:"@@"`
	Right []*AndRHS  `parser:"@@*"`
}

func (e *AndExpression) Evaluate(b *QueryBuilder) string {
	leftTable := e.Left.Evaluate(b)
	tableName := leftTable

	for _, rhs := range e.Right {
		rightTable := rhs.Evaluate(b)
		tableName = b.nextTableName()

		b.writeComma()

		b.tableBuilder.WriteString(tableName)
		b.tableBuilder.WriteString(" AS (")
		b.tableBuilder.WriteString("SELECT * FROM ")
		b.tableBuilder.WriteString(leftTable)
		b.tableBuilder.WriteString(" INTERSECT ")
		b.tableBuilder.WriteString("SELECT * FROM ")
		b.tableBuilder.WriteString(rightTable)
		b.tableBuilder.WriteString(")")

		leftTable = rightTable
	}

	return tableName
}

// AndRHS represents the right-hand side of an AND.
type AndRHS struct {
	Expr *EqualExpr `parser:"And @@"`
}

func (e *AndRHS) Evaluate(b *QueryBuilder) string {
	return e.Expr.Evaluate(b)
}

// EqualExpr can be either an equality or a parenthesized expression.
type EqualExpr struct {
	Paren  *Expression `parser:"  \"(\" @@ \")\""`
	Owner  *Ownership  `parser:"| @@"`
	Assign *Equality   `parser:"| @@"`

	LessThan           *LessThan           `parser:"| @@"`
	LessOrEqualThan    *LessOrEqualThan    `parser:"| @@"`
	GreaterThan        *GreaterThan        `parser:"| @@"`
	GreaterOrEqualThan *GreaterOrEqualThan `parser:"| @@"`
	Glob               *Glob               `parser:"| @@"`
}

func (e *EqualExpr) Evaluate(b *QueryBuilder) string {
	if e.Paren != nil {
		return e.Paren.Recurse(b)
	}

	if e.Owner != nil {
		return e.Owner.Evaluate(b)
	}

	if e.LessThan != nil {
		return e.LessThan.Evaluate(b)
	}

	if e.LessOrEqualThan != nil {
		return e.LessOrEqualThan.Evaluate(b)
	}

	if e.GreaterThan != nil {
		return e.GreaterThan.Evaluate(b)
	}

	if e.GreaterOrEqualThan != nil {
		return e.GreaterOrEqualThan.Evaluate(b)
	}

	if e.Glob != nil {
		return e.Glob.Evaluate(b)
	}

	if e.Assign != nil {
		return e.Assign.Evaluate(b)
	}

	panic("This should not happen!")
}

type Glob struct {
	Var   string `parser:"@Ident Glob"`
	Value string `parser:"@String"`
}

func (e *Glob) Evaluate(b *QueryBuilder) string {
	return b.createLeafQuery(
		"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value GLOB ?",
		e.Var, e.Value,
	)
}

type LessThan struct {
	Var   string `parser:"@Ident Lt"`
	Value *Value `parser:"@@"`
}

func (e *LessThan) Evaluate(b *QueryBuilder) string {
	if e.Value.String != nil {
		return b.createLeafQuery(
			"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value < ?",
			e.Var, *e.Value.String,
		)
	} else {
		return b.createLeafQuery(
			"SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value < ?",
			e.Var, *e.Value.Number,
		)
	}
}

type LessOrEqualThan struct {
	Var   string `parser:"@Ident Leqt"`
	Value *Value `parser:"@@"`
}

func (e *LessOrEqualThan) Evaluate(b *QueryBuilder) string {
	if e.Value.String != nil {
		return b.createLeafQuery(
			"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value <= ?",
			e.Var, *e.Value.String,
		)
	} else {
		return b.createLeafQuery(
			"SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value <= ?",
			e.Var, *e.Value.Number,
		)
	}
}

type GreaterThan struct {
	Var   string `parser:"@Ident Gt"`
	Value *Value `parser:"@@"`
}

func (e *GreaterThan) Evaluate(b *QueryBuilder) string {
	if e.Value.String != nil {
		return b.createLeafQuery(
			"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value > ?",
			e.Var, *e.Value.String,
		)
	} else {
		return b.createLeafQuery(
			"SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value > ?",
			e.Var, *e.Value.Number,
		)
	}
}

type GreaterOrEqualThan struct {
	Var   string `parser:"@Ident Geqt"`
	Value *Value `parser:"@@"`
}

func (e *GreaterOrEqualThan) Evaluate(b *QueryBuilder) string {
	if e.Value.String != nil {
		return b.createLeafQuery(
			"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value >= ?",
			e.Var, *e.Value.String,
		)
	} else {
		return b.createLeafQuery(
			"SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value >= ?",
			e.Var, *e.Value.Number,
		)
	}
}

// Ownership represents an ownership query, $owner = 0x....
type Ownership struct {
	Owner string `parser:"Owner Eq @String"`
}

func (e *Ownership) Evaluate(b *QueryBuilder) string {
	var address = common.Address{}
	if common.IsHexAddress(e.Owner) {
		address = common.HexToAddress(e.Owner)
	}
	return b.createLeafQuery(
		"SELECT key FROM entities WHERE owner_address = ?",
		address.Hex(),
	)
}

// Equality represents a simple equality (e.g. name = 123).
type Equality struct {
	Var   string `parser:"@Ident \"=\""`
	Value *Value `parser:"@@"`
}

func (e *Equality) Evaluate(b *QueryBuilder) string {
	if e.Value.String != nil {
		return b.createLeafQuery(
			"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value = ?",
			e.Var, *e.Value.String,
		)
	} else {
		return b.createLeafQuery(
			"SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value = ?",
			e.Var, *e.Value.Number,
		)
	}
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
