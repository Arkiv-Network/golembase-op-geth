package arkivtype

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

type QueryResponse struct {
	Data        []json.RawMessage `json:"data"`
	BlockNumber uint64            `json:"blockNumber"`
	Cursor      string            `json:"cursor,omitempty"`
}

type Offset []OffsetValue

func (o Offset) Encode() (string, error) {
	s, err := json.Marshal(o)
	if err != nil {
		return "", fmt.Errorf("could not marshal offset: %w", err)
	}

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

	err = json.Unmarshal(bs, o)
	if err != nil {
		return fmt.Errorf("could not unmarshal offset: %w (%s)", err, string(bs))
	}

	return nil
}

type OffsetValue struct {
	ColumnName string `json:"columnName"`
	Value      any    `json:"value"`
}

type EntityData struct {
	Key         common.Hash    `json:"key,omitempty"`
	Value       hexutil.Bytes  `json:"value,omitempty"`
	ContentType string         `json:"contentType,omitempty"`
	ExpiresAt   uint64         `json:"expiresAt,omitempty"`
	Owner       common.Address `json:"owner,omitempty"`

	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations,omitempty"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations,omitempty"`
}
