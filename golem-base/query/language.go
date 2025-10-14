package query

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

// Define the lexer with distinct tokens for each operator and parentheses.
var lex = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Whitespace", Pattern: `[ \t\n\r]+`},
	{Name: "LParen", Pattern: `\(`},
	{Name: "RParen", Pattern: `\)`},
	{Name: "And", Pattern: `&&`},
	{Name: "Or", Pattern: `\|\|`},
	{Name: "Neq", Pattern: `!=`},
	{Name: "Eq", Pattern: `=`},
	{Name: "Geqt", Pattern: `>=`},
	{Name: "Leqt", Pattern: `<=`},
	{Name: "Gt", Pattern: `>`},
	{Name: "Lt", Pattern: `<`},
	{Name: "NotGlob", Pattern: `!~`},
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
	Or OrExpression `parser:"@@"`
}

func (e *Expression) Normalise() *Expression {
	normalised := e.Or.Normalise()
	// Remove unneeded OR+AND nodes that both only contain a single child
	// when that child is a parenthesised expression
	if len(normalised.Right) == 0 && len(normalised.Left.Right) == 0 && normalised.Left.Left.Paren != nil {
		// This has already been normalised by the call above, so any negation has
		// been pushed into the leaf expressions and we can safely strip away the
		// parentheses
		return &normalised.Left.Left.Paren.Nested
	}
	return &Expression{
		Or: *normalised,
	}
}

func (e *Expression) invert() *Expression {

	newLeft := e.Or.invert()

	if len(newLeft.Right) == 0 {
		// By construction, this will always be a Paren
		if newLeft.Left.Paren == nil {
			panic("This should never happen!")
		}
		return &newLeft.Left.Paren.Nested
	}

	return &Expression{
		Or: OrExpression{
			Left: *newLeft,
		},
	}
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

	tableBuilder.WriteString(" SELECT entity_key FROM ")
	tableBuilder.WriteString(tableName)
	// tableBuilder.WriteString(", entities LEFT JOIN ")
	tableBuilder.WriteString(" ORDER BY 1")

	return &SelectQuery{
		Query: tableBuilder.String(),
		Args:  builder.args,
	}
}

// OrExpression handles expressions connected with ||.
type OrExpression struct {
	Left  AndExpression `parser:"@@"`
	Right []*OrRHS      `parser:"@@*"`
}

func (e *OrExpression) Normalise() *OrExpression {
	var newRight []*OrRHS = nil

	if e.Right != nil {
		newRight = make([]*OrRHS, 0, len(e.Right))
		for _, rhs := range e.Right {
			newRight = append(newRight, rhs.Normalise())
		}
	}

	return &OrExpression{
		Left:  *e.Left.Normalise(),
		Right: newRight,
	}
}

func (e *OrExpression) invert() *AndExpression {
	newLeft := EqualExpr{
		Paren: &Paren{
			IsNot: false,
			Nested: Expression{
				Or: *e.Left.invert(),
			},
		},
	}

	var newRight []*AndRHS = nil

	if e.Right != nil {
		newRight = make([]*AndRHS, 0, len(e.Right))
		for _, rhs := range e.Right {
			newRight = append(newRight, rhs.invert())
		}
	}

	return &AndExpression{
		Left:  newLeft,
		Right: newRight,
	}
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
		b.tableBuilder.WriteString("SELECT entity_key FROM ")
		b.tableBuilder.WriteString(leftTable)
		b.tableBuilder.WriteString(" UNION ")
		b.tableBuilder.WriteString("SELECT entity_key FROM ")
		b.tableBuilder.WriteString(rightTable)
		b.tableBuilder.WriteString(")")

		// Carry forward the cumulative result of the UNION
		leftTable = tableName
	}

	return tableName
}

// OrRHS represents the right-hand side of an OR.
type OrRHS struct {
	Expr AndExpression `parser:"Or @@"`
}

func (e *OrRHS) Normalise() *OrRHS {
	return &OrRHS{
		Expr: *e.Expr.Normalise(),
	}
}

func (e *OrRHS) invert() *AndRHS {
	return &AndRHS{
		Expr: EqualExpr{
			Paren: &Paren{
				IsNot: false,
				Nested: Expression{
					Or: *e.Expr.invert(),
				},
			},
		},
	}
}

func (e *OrRHS) Evaluate(b *QueryBuilder) string {
	return e.Expr.Evaluate(b)
}

// AndExpression handles expressions connected with &&.
type AndExpression struct {
	Left  EqualExpr `parser:"@@"`
	Right []*AndRHS `parser:"@@*"`
}

func (e *AndExpression) Normalise() *AndExpression {
	var newRight []*AndRHS = nil

	if e.Right != nil {
		newRight = make([]*AndRHS, 0, len(e.Right))
		for _, rhs := range e.Right {
			newRight = append(newRight, rhs.Normalise())
		}
	}

	return &AndExpression{
		Left:  *e.Left.Normalise(),
		Right: newRight,
	}
}

func (e *AndExpression) invert() *OrExpression {
	newLeft := AndExpression{
		Left: *e.Left.invert(),
	}

	var newRight []*OrRHS = nil

	if e.Right != nil {
		newRight = make([]*OrRHS, 0, len(e.Right))
		for _, rhs := range e.Right {
			newRight = append(newRight, rhs.invert())
		}
	}

	return &OrExpression{
		Left:  newLeft,
		Right: newRight,
	}
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
		b.tableBuilder.WriteString("SELECT entity_key FROM ")
		b.tableBuilder.WriteString(leftTable)
		b.tableBuilder.WriteString(" INTERSECT ")
		b.tableBuilder.WriteString("SELECT entity_key FROM ")
		b.tableBuilder.WriteString(rightTable)
		b.tableBuilder.WriteString(")")

		// Carry forward the cumulative result of the INTERSECT
		leftTable = tableName
	}

	return tableName
}

// AndRHS represents the right-hand side of an AND.
type AndRHS struct {
	Expr EqualExpr `parser:"And @@"`
}

func (e *AndRHS) Normalise() *AndRHS {
	return &AndRHS{
		Expr: *e.Expr.Normalise(),
	}
}

func (e *AndRHS) invert() *OrRHS {
	return &OrRHS{
		Expr: AndExpression{
			Left: *e.Expr.invert(),
		},
	}
}

func (e *AndRHS) Evaluate(b *QueryBuilder) string {
	return e.Expr.Evaluate(b)
}

// EqualExpr can be either an equality or a parenthesized expression.
type EqualExpr struct {
	Paren  *Paren     `parser:"  @@"`
	Owner  *Ownership `parser:"| @@"`
	Assign *Equality  `parser:"| @@"`

	LessThan           *LessThan           `parser:"| @@"`
	LessOrEqualThan    *LessOrEqualThan    `parser:"| @@"`
	GreaterThan        *GreaterThan        `parser:"| @@"`
	GreaterOrEqualThan *GreaterOrEqualThan `parser:"| @@"`
	Glob               *Glob               `parser:"| @@"`
}

func (e *EqualExpr) Normalise() *EqualExpr {
	normalised := e

	if e.Paren != nil {
		p := e.Paren.Normalise()

		// Remove parentheses that only contain a single nested expression
		// (i.e. no OR or AND with multiple children)
		if len(p.Nested.Or.Right) == 0 && len(p.Nested.Or.Left.Right) == 0 {
			// This expression should already be properly normalised, we don't need to
			// call Normalise again here
			normalised = &p.Nested.Or.Left.Left
		} else {
			normalised = &EqualExpr{Paren: p}
		}
	}

	// Everything other than parenthesised expressions do not require further normalisation
	return normalised
}

func (e *EqualExpr) invert() *EqualExpr {
	if e.Paren != nil {
		return &EqualExpr{Paren: e.Paren.invert()}
	}

	if e.Owner != nil {
		return &EqualExpr{Owner: e.Owner.invert()}
	}

	if e.LessThan != nil {
		return &EqualExpr{GreaterOrEqualThan: e.LessThan.invert()}
	}

	if e.LessOrEqualThan != nil {
		return &EqualExpr{GreaterThan: e.LessOrEqualThan.invert()}
	}

	if e.GreaterThan != nil {
		return &EqualExpr{LessOrEqualThan: e.GreaterThan.invert()}
	}

	if e.GreaterOrEqualThan != nil {
		return &EqualExpr{LessThan: e.GreaterOrEqualThan.invert()}
	}

	if e.Glob != nil {
		return &EqualExpr{Glob: e.Glob.invert()}
	}

	if e.Assign != nil {
		return &EqualExpr{Assign: e.Assign.invert()}
	}

	panic("This should not happen!")
}

func (e *EqualExpr) Evaluate(b *QueryBuilder) string {
	if e.Paren != nil {
		return e.Paren.Evaluate(b)
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

type Paren struct {
	IsNot  bool       `parser:"@Not?"`
	Nested Expression `parser:"LParen @@ RParen"`
}

func (e *Paren) Normalise() *Paren {
	nested := e.Nested

	if e.IsNot {
		nested = *nested.invert()
	}

	return &Paren{
		IsNot:  false,
		Nested: *nested.Normalise(),
	}
}

func (e *Paren) invert() *Paren {
	return &Paren{
		IsNot:  !e.IsNot,
		Nested: e.Nested,
	}
}

func (e *Paren) Evaluate(b *QueryBuilder) string {
	expr := e.Nested
	// If we have a negation, we will push it down into the expression
	if e.IsNot {
		expr = *e.Nested.invert()
	}
	// We don't have to do anything here regarding precedence, the parsing order
	// is already taking care of precedence since the nested OR node will create a subquery
	return expr.Or.Evaluate(b)
}

type Glob struct {
	Var   string `parser:"@Ident"`
	IsNot bool   `parser:"(Glob | @NotGlob)"`
	Value string `parser:"@String"`
}

func (e *Glob) invert() *Glob {
	return &Glob{
		Var:   e.Var,
		IsNot: !e.IsNot,
		Value: e.Value,
	}
}

func (e *Glob) Evaluate(b *QueryBuilder) string {
	if !e.IsNot {
		return b.createLeafQuery(
			"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value GLOB ?",
			e.Var, e.Value,
		)
	} else {
		return b.createLeafQuery(
			"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value NOT GLOB ?",
			e.Var, e.Value,
		)
	}
}

type LessThan struct {
	Var   string `parser:"@Ident Lt"`
	Value Value  `parser:"@@"`
}

func (e *LessThan) invert() *GreaterOrEqualThan {
	return &GreaterOrEqualThan{
		Var:   e.Var,
		Value: e.Value,
	}
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
	Value Value  `parser:"@@"`
}

func (e *LessOrEqualThan) invert() *GreaterThan {
	return &GreaterThan{
		Var:   e.Var,
		Value: e.Value,
	}
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
	Value Value  `parser:"@@"`
}

func (e *GreaterThan) invert() *LessOrEqualThan {
	return &LessOrEqualThan{
		Var:   e.Var,
		Value: e.Value,
	}
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
	Value Value  `parser:"@@"`
}

func (e *GreaterOrEqualThan) invert() *LessThan {
	return &LessThan{
		Var:   e.Var,
		Value: e.Value,
	}
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
	IsNot bool   `parser:"Owner (Eq | @Neq)"`
	Owner string `parser:"@String"`
}

func (e *Ownership) invert() *Ownership {
	return &Ownership{
		IsNot: !e.IsNot,
		Owner: e.Owner,
	}
}

func (e *Ownership) Evaluate(b *QueryBuilder) string {
	var address = common.Address{}
	if common.IsHexAddress(e.Owner) {
		address = common.HexToAddress(e.Owner)
	}
	if !e.IsNot {
		return b.createLeafQuery(
			"SELECT key AS entity_key FROM entities WHERE owner_address = ?",
			address.Hex(),
		)
	} else {
		return b.createLeafQuery(
			"SELECT key AS entity_key FROM entities WHERE owner_address != ?",
			address.Hex(),
		)
	}
}

// Equality represents a simple equality (e.g. name = 123).
type Equality struct {
	Var   string `parser:"@Ident"`
	IsNot bool   `parser:"(Eq | @Neq)"`
	Value Value  `parser:"@@"`
}

func (e *Equality) invert() *Equality {
	return &Equality{
		Var:   e.Var,
		IsNot: !e.IsNot,
		Value: e.Value,
	}
}

func (e *Equality) Evaluate(b *QueryBuilder) string {
	if !e.IsNot {
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
	} else {
		if e.Value.String != nil {
			return b.createLeafQuery(
				"SELECT entity_key FROM string_annotations WHERE annotation_key = ? AND value != ?",
				e.Var, *e.Value.String,
			)
		} else {
			return b.createLeafQuery(
				"SELECT entity_key FROM numeric_annotations WHERE annotation_key = ? AND value != ?",
				e.Var, *e.Value.Number,
			)
		}
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
	log.Debug("Parsing query", "query", s)

	v, err := Parser.ParseString("", s)
	if err != nil {
		return nil, err
	}
	return v.Normalise(), err
}
