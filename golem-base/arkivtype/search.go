package arkivtype

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
)

var allColumns []string = []string{
	"key",
	"payload",
	"content_type",
	"expires_at",
	"owner_address",
	"last_modified_at_block",
	"transaction_index_in_block",
	"operation_index_in_transaction",
}

type columnEntry struct {
	Index int
	Name  string
}

// allColumnsMapping is used to verify user-supplied columns to protect against SQL injection
var allColumnsMapping map[string]columnEntry = make(map[string]columnEntry)

func init() {
	for i, column := range allColumns {
		allColumnsMapping[column] = columnEntry{
			Index: i,
			Name:  column,
		}
	}
}

func getColumn(name string) (*columnEntry, error) {
	column, exists := allColumnsMapping[name]
	if !exists {
		return nil, fmt.Errorf("invalid column name: %s", column.Name)
	}
	return &column, nil
}

func GetColumnIndex(column string) (int, error) {
	columnEntry, err := getColumn(column)
	if err != nil {
		return -1, fmt.Errorf("unknown column %s", column)
	}
	return columnEntry.Index, nil
}

func GetColumn(name string) (string, error) {
	column, err := getColumn(name)
	if err != nil {
		return "", err
	}
	return column.Name, nil
}

// GetColumnOrPanic is used for non user-supplied columns, to detect wrong literals early
func GetColumnOrPanic(name string) string {
	column, err := getColumn(name)
	if err != nil {
		panic(fmt.Sprintf("invalid column name: %s", column.Name))
	}
	return column.Name
}

type QueryResponse struct {
	Data        []json.RawMessage `json:"data"`
	BlockNumber uint64            `json:"blockNumber"`
	Cursor      *string           `json:"cursor,omitempty"`
}

type Offset struct {
	BlockNumber  uint64        `json:"blockNumber"`
	ColumnValues []OffsetValue `json:"columnValues"`
}

func (o Offset) Encode() (string, error) {

	encodedOffset := encodedOffset{
		BlockNumber:  o.BlockNumber,
		ColumnValues: make([][]any, 0, len(o.ColumnValues)),
	}

	for _, c := range o.ColumnValues {
		columnIx, err := GetColumnIndex(c.ColumnName)
		if err != nil {
			return "", err
		}
		encodedOffset.ColumnValues = append(encodedOffset.ColumnValues, []any{
			columnIx, c.Value,
		})
	}

	s, err := json.Marshal(encodedOffset)
	if err != nil {
		return "", fmt.Errorf("could not marshal offset: %w", err)
	}

	log.Info("query response", "encoded cursor", string(s))

	return hex.EncodeToString([]byte(s)), nil
}

func (o *Offset) Decode(s string) error {
	if len(s) == 0 {
		return nil
	}

	bs, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("could not decode offset: %w", err)
	}

	encoded := encodedOffset{}
	err = json.Unmarshal(bs, &encoded)
	if err != nil {
		return fmt.Errorf("could not unmarshal offset: %w (%s)", err, string(bs))
	}

	o.BlockNumber = encoded.BlockNumber
	o.ColumnValues = make([]OffsetValue, 0, len(encoded.ColumnValues))

	for _, c := range encoded.ColumnValues {
		if len(c) != 2 {
			return fmt.Errorf("invalid length of cursor array: %d", len(c))
		}

		firstValue, ok := c[0].(float64)
		if !ok {
			return fmt.Errorf("unknown column index: %d", c[0])
		}

		columnIx := int(firstValue)
		if columnIx >= len(allColumns) {
			return fmt.Errorf("unknown column index: %d", columnIx)
		}

		o.ColumnValues = append(o.ColumnValues, OffsetValue{
			ColumnName: allColumns[columnIx],
			Value:      c[1],
		})
	}

	return nil
}

type OffsetValue struct {
	ColumnName string `json:"columnName"`
	Value      any    `json:"value"`
}

// type to encode the cursor in a small json document to avoid overhead
type encodedOffset struct {
	BlockNumber  uint64  `json:"b"`
	ColumnValues [][]any `json:"v"`
}

type EntityData struct {
	Key         *common.Hash    `json:"key,omitempty"`
	Value       hexutil.Bytes   `json:"value,omitempty"`
	ContentType *string         `json:"contentType,omitempty"`
	ExpiresAt   *uint64         `json:"expiresAt,omitempty"`
	Owner       *common.Address `json:"owner,omitempty"`

	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations,omitempty"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations,omitempty"`
}
