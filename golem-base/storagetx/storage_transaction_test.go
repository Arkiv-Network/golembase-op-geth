package storagetx_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageTransactionMarshalling(t *testing.T) {
	t.Run("FullyPopulatedTransaction", func(t *testing.T) {
		// Create a sample transaction with all fields populated
		tx := &storagetx.StorageTransaction{
			Create: []storagetx.Create{
				{
					BTL:     100,
					Payload: []byte("test payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "type", Value: "test"},
						{Key: "name", Value: "example"},
					},
					NumericAnnotations: []entity.NumericAnnotation{
						{Key: "version", Value: 1},
						{Key: "size", Value: 1024},
					},
				},
			},
			Update: []storagetx.Update{
				{
					EntityKey: common.HexToHash("0x1234567890"),
					BTL:       200,
					Payload:   []byte("updated payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "status", Value: "updated"},
					},
					NumericAnnotations: []entity.NumericAnnotation{
						{Key: "timestamp", Value: 1678901234},
					},
				},
			},
			Delete: []common.Hash{
				common.HexToHash("0xdeadbeef"),
				common.HexToHash("0xbeefdead"),
			},
		}

		// Test marshalling
		encoded, err := rlp.EncodeToBytes(tx)
		require.NoError(t, err)
		require.NotEmpty(t, encoded)

		// Test unmarshalling
		var decoded storagetx.StorageTransaction
		err = rlp.DecodeBytes(encoded, &decoded)
		require.NoError(t, err)

		// Verify all fields match
		assert.Equal(t, tx.Create[0].BTL, decoded.Create[0].BTL)
		assert.Equal(t, tx.Create[0].Payload, decoded.Create[0].Payload)
		assert.Equal(t, tx.Create[0].StringAnnotations, decoded.Create[0].StringAnnotations)
		assert.Equal(t, tx.Create[0].NumericAnnotations, decoded.Create[0].NumericAnnotations)

		assert.Equal(t, tx.Update[0].EntityKey, decoded.Update[0].EntityKey)
		assert.Equal(t, tx.Update[0].BTL, decoded.Update[0].BTL)
		assert.Equal(t, tx.Update[0].Payload, decoded.Update[0].Payload)
		assert.Equal(t, tx.Update[0].StringAnnotations, decoded.Update[0].StringAnnotations)
		assert.Equal(t, tx.Update[0].NumericAnnotations, decoded.Update[0].NumericAnnotations)

		assert.Equal(t, tx.Delete, decoded.Delete)
	})

	t.Run("EmptyTransaction", func(t *testing.T) {
		// Test empty transaction
		emptyTx := &storagetx.StorageTransaction{}
		encoded, err := rlp.EncodeToBytes(emptyTx)
		require.NoError(t, err)

		var decodedEmpty storagetx.StorageTransaction
		err = rlp.DecodeBytes(encoded, &decodedEmpty)
		require.NoError(t, err)

		assert.Empty(t, decodedEmpty.Create)
		assert.Empty(t, decodedEmpty.Update)
		assert.Empty(t, decodedEmpty.Delete)
	})
}
