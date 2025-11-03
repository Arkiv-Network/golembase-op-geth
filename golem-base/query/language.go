package query

import (
	"fmt"
	"slices"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

type QueryOptions struct {
	AtBlock            uint64                  `json:"atBlock"`
	IncludeAnnotations bool                    `json:"includeAnnotations"`
	Columns            []string                `json:"columns"`
	Offset             []arkivtype.OffsetValue `json:"offset"`
}

func (opts *QueryOptions) AllColumns() []string {
	return append(opts.Columns, opts.OrderByColumns()...)
}

func (opts *QueryOptions) OrderByColumns() []string {
	return []string{
		arkivtype.GetColumnOrPanic("last_modified_at_block"),
		arkivtype.GetColumnOrPanic("transaction_index_in_block"),
		arkivtype.GetColumnOrPanic("operation_index_in_transaction"),
	}
}

func (opts *QueryOptions) columnString() string {
	if len(opts.AllColumns()) == 0 {
		return "1"
	}
	return strings.Join(opts.AllColumns(), ", ")
}

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
	{Name: "EntityKey", Pattern: `0x[a-fA-F0-9]{64}`},
	{Name: "Address", Pattern: `0x[a-fA-F0-9]{40}`},
	{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
	{Name: "Number", Pattern: `[0-9]+`},
	{Name: "Ident", Pattern: entity.AnnotationIdentRegex},
	// Meta-annotations, should start with $
	{Name: "Owner", Pattern: `\$owner`},
	{Name: "Key", Pattern: `\$key`},
	{Name: "Expiration", Pattern: `\$expiration`},
	{Name: "All", Pattern: `\$all`},
})

type SelectQuery struct {
	Query   string
	Args    []any
	Columns []string
}

type QueryBuilder struct {
	tableBuilder *strings.Builder
	args         []any
	needsComma   bool
	tableCounter uint64
	options      QueryOptions
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

func (b *QueryBuilder) getPaginationArguments() (string, []any) {
	args := []any{}
	paginationConditions := []string{}

	for i := range b.options.Offset {
		subcondition := []string{}
		for j, from := range b.options.Offset {
			if j > i {
				break
			}
			var operator string
			if j < i {
				operator = "="
			} else {
				// TODO: if we ever support DESC, we need to optionally invert this
				operator = ">"
			}

			args = append(args, from.Value)

			subcondition = append(
				subcondition,
				fmt.Sprintf("%s %s ?", from.ColumnName, operator),
			)
		}

		paginationConditions = append(
			paginationConditions,
			fmt.Sprintf("(%s)", strings.Join(subcondition, " AND ")),
		)
	}

	paginationCondition := strings.Join(paginationConditions, " OR ")
	return paginationCondition, args
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

type TopLevel struct {
	Expression *Expression `parser:"@@"`
	All        bool        `parser:"| @(All | '*')"`
}

func (t *TopLevel) Normalise() *TopLevel {
	if t.All {
		return t
	}
	return &TopLevel{
		Expression: t.Expression.Normalise(),
		All:        t.All,
	}
}

func (t *TopLevel) Evaluate(options QueryOptions) *SelectQuery {
	tableBuilder := strings.Builder{}
	args := []any{}

	builder := QueryBuilder{
		options:      options,
		tableBuilder: &tableBuilder,
		args:         args,
		needsComma:   false,
	}

	if t.All {
		builder.tableBuilder.WriteString(" SELECT DISTINCT ")
		builder.tableBuilder.WriteString(builder.options.columnString())
		builder.tableBuilder.WriteString(" FROM entities ORDER BY ")
		builder.tableBuilder.WriteString(strings.Join(builder.options.OrderByColumns(), ", "))
	} else {
		t.Expression.Evaluate(&builder)
	}

	return &SelectQuery{
		Query:   builder.tableBuilder.String(),
		Args:    builder.args,
		Columns: builder.options.AllColumns(),
	}
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

func (e *Expression) Evaluate(builder *QueryBuilder) {
	builder.tableBuilder.WriteString("WITH ")

	tableName := e.Or.Evaluate(builder)

	paginationCondition, paginationArgs := builder.getPaginationArguments()

	builder.args = append(builder.args, paginationArgs...)

	p := ""
	if len(paginationCondition) > 0 {
		p = fmt.Sprintf(" WHERE ( %s )", paginationCondition)
	}

	builder.tableBuilder.WriteString(" SELECT DISTINCT * FROM ")
	builder.tableBuilder.WriteString(tableName)
	builder.tableBuilder.WriteString(p)
	builder.tableBuilder.WriteString(" ORDER BY ")
	builder.tableBuilder.WriteString(strings.Join(builder.options.OrderByColumns(), ", "))
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
		b.tableBuilder.WriteString("SELECT * FROM ")
		b.tableBuilder.WriteString(leftTable)
		b.tableBuilder.WriteString(" UNION ")
		b.tableBuilder.WriteString("SELECT * FROM ")
		b.tableBuilder.WriteString(rightTable)
		b.tableBuilder.WriteString(")")

		// Carry forward the cumulative result of the UNION
		leftTable = tableName
	}

	return tableName
}

// OrRHS represents the right-hand side of an OR.
type OrRHS struct {
	Expr AndExpression `parser:"(Or | 'OR' | 'or') @@"`
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
		b.tableBuilder.WriteString("SELECT * FROM ")
		b.tableBuilder.WriteString(leftTable)
		b.tableBuilder.WriteString(" INTERSECT ")
		b.tableBuilder.WriteString("SELECT * FROM ")
		b.tableBuilder.WriteString(rightTable)
		b.tableBuilder.WriteString(")")

		// Carry forward the cumulative result of the INTERSECT
		leftTable = tableName
	}

	return tableName
}

// AndRHS represents the right-hand side of an AND.
type AndRHS struct {
	Expr EqualExpr `parser:"(And | 'AND' | 'and') @@"`
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
	Paren     *Paren     `parser:"  @@"`
	Assign    *Equality  `parser:"| @@"`
	Inclusion *Inclusion `parser:"| @@"`

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

	if e.Inclusion != nil {
		return &EqualExpr{Inclusion: e.Inclusion.invert()}
	}

	panic("This should not happen!")
}

func (e *EqualExpr) Evaluate(b *QueryBuilder) string {
	if e.Paren != nil {
		return e.Paren.Evaluate(b)
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

	if e.Inclusion != nil {
		return e.Inclusion.Evaluate(b)
	}

	panic("This should not happen!")
}

type Paren struct {
	IsNot  bool       `parser:"@(Not | 'NOT' | 'not')?"`
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

func (b *QueryBuilder) createAnnotationQuery(
	tableName string,
	whereClause string,
	arguments ...any,
) string {
	args := make([]any, 0, len(arguments)+2)
	args = append(args, b.options.AtBlock, b.options.AtBlock)
	args = append(args, arguments...)

	return b.createLeafQuery(
		strings.Join(
			[]string{
				"SELECT DISTINCT",
				b.options.columnString(),
				"FROM",
				tableName,
				"AS a INNER JOIN entities AS e",
				"ON a.entity_key = e.key",
				"AND a.entity_last_modified_at_block = e.last_modified_at_block",
				"AND e.deleted = FALSE",
				"AND e.last_modified_at_block <= ?",
				"AND NOT EXISTS (",
				"SELECT 1",
				"FROM entities AS e2",
				"WHERE e2.key = e.key",
				"AND e2.last_modified_at_block > e.last_modified_at_block",
				"AND e2.last_modified_at_block <= ?",
				")",
				"WHERE",
				whereClause,
			},
			" ",
		),
		args...,
	)
}

type Glob struct {
	Var   string `parser:"@Ident"`
	IsNot bool   `parser:"((Glob | @NotGlob) | (@('NOT' | 'not')? ('GLOB' | 'glob')))"`
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
		return b.createAnnotationQuery(
			"string_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value GLOB ?",
				},
				" ",
			),
			e.Var,
			e.Value,
		)
	} else {
		return b.createAnnotationQuery(
			"string_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value NOT GLOB ?",
				},
				" ",
			),
			e.Var,
			e.Value,
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
		return b.createAnnotationQuery(
			"string_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value < ?",
				},
				" ",
			),
			e.Var,
			*e.Value.String,
		)
	} else {
		return b.createAnnotationQuery(
			"numeric_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value < ?",
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
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
		return b.createAnnotationQuery(
			"string_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value <= ?",
				},
				" ",
			),
			e.Var,
			*e.Value.String,
		)
	} else {
		return b.createAnnotationQuery(
			"numeric_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value <= ?",
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
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
		return b.createAnnotationQuery(
			"string_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value > ?",
				},
				" ",
			),
			e.Var,
			*e.Value.String,
		)
	} else {
		return b.createAnnotationQuery(
			"numeric_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value > ?",
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
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
		return b.createAnnotationQuery(
			"string_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value >= ?",
				},
				" ",
			),
			e.Var,
			*e.Value.String,
		)
	} else {
		return b.createAnnotationQuery(
			"numeric_annotations",
			strings.Join(
				[]string{
					"annotation_key = ?",
					"AND value >= ?",
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
		)
	}
}

// Equality represents a simple equality (e.g. name = 123).
type Equality struct {
	Var   string `parser:"(@Ident | @Key | @Owner | @Expiration)"`
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
	if e.Value.String != nil {

		value := *e.Value.String
		if e.Var == "$owner" || e.Var == "$key" {
			value = strings.ToLower(value)
		}

		condition := "a.value = ?"
		if e.IsNot {
			condition = "a.value != ?"
		}

		return b.createAnnotationQuery(
			"string_annotations",
			strings.Join(
				[]string{
					"a.annotation_key = ?",
					"AND",
					condition,
				},
				" ",
			),
			e.Var,
			value,
		)

	} else {

		condition := "a.value = ?"
		if e.IsNot {
			condition = "a.value != ?"
		}

		return b.createAnnotationQuery(
			"numeric_annotations",
			strings.Join(
				[]string{
					"a.annotation_key = ?",
					"AND",
					condition,
				},
				" ",
			),
			e.Var,
			*e.Value.Number,
		)

	}
}

type Inclusion struct {
	Var    string `parser:"(@Ident | @Key | @Owner | @Expiration)"`
	IsNot  bool   `parser:"(@('NOT'|'not')? ('IN'|'in'))"`
	Values Values `parser:"@@"`
}

func (e *Inclusion) invert() *Inclusion {
	return &Inclusion{
		Var:    e.Var,
		IsNot:  !e.IsNot,
		Values: e.Values,
	}
}

func (e *Inclusion) Evaluate(b *QueryBuilder) string {
	if len(e.Values.Strings) > 0 {

		values := make([]any, 0, len(e.Values.Strings)+1)
		values = append(values, e.Var)
		for _, value := range e.Values.Strings {
			if e.Var == "$owner" || e.Var == "$key" {
				values = append(values, strings.ToLower(value))
			} else {
				values = append(values, value)
			}
		}

		paramStr := strings.Join(slices.Repeat([]string{"?"}, len(e.Values.Strings)), ", ")

		condition := fmt.Sprintf("a.value IN (%s)", paramStr)
		if e.IsNot {
			condition = fmt.Sprintf("a.value NOT IN (%s)", paramStr)
		}

		return b.createAnnotationQuery(
			"string_annotations",
			strings.Join(
				[]string{
					"a.annotation_key = ?",
					"AND",
					condition,
				},
				" ",
			),
			values...,
		)

	} else {

		values := make([]any, 0, len(e.Values.Numbers)+1)
		values = append(values, e.Var)
		for _, value := range e.Values.Numbers {
			values = append(values, value)
		}

		paramStr := strings.Join(slices.Repeat([]string{"?"}, len(e.Values.Numbers)), ", ")

		condition := fmt.Sprintf("a.value IN (%s)", paramStr)
		if e.IsNot {
			condition = fmt.Sprintf("a.value NOT IN (%s)", paramStr)
		}

		return b.createAnnotationQuery(
			"numeric_annotations",
			strings.Join(
				[]string{
					"a.annotation_key = ?",
					"AND",
					condition,
				},
				" ",
			),
			values...,
		)

	}
}

// Value is a literal value (a number or a string).
type Value struct {
	String *string `parser:"  (@String | @EntityKey | @Address)"`
	Number *uint64 `parser:"| @Number"`
}

type Values struct {
	Strings []string `parser:"  '(' (@String | @EntityKey | @Address)+ ')'"`
	Numbers []uint64 `parser:"| '(' @Number+ ')'"`
}

var Parser = participle.MustBuild[TopLevel](
	participle.Lexer(lex),
	participle.Elide("Whitespace"),
	participle.Unquote("String"),
)

func Parse(s string) (*TopLevel, error) {
	log.Info("Parsing query", "query", s)

	v, err := Parser.ParseString("", s)
	if err != nil {
		return nil, err
	}
	return v.Normalise(), err
}
