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
		assert.Empty(t, decodedEmpty.Extend)
	})

	t.Run("TransactionWithExtendBTL", func(t *testing.T) {
		// Test transaction with ExtendBTL operations
		tx := &storagetx.StorageTransaction{
			Extend: []storagetx.ExtendBTL{
				{
					EntityKey:      common.HexToHash("0x1234567890abcdef"),
					NumberOfBlocks: 500,
				},
				{
					EntityKey:      common.HexToHash("0xabcdef1234567890"),
					NumberOfBlocks: 1000,
				},
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

		// Verify ExtendBTL fields match
		require.Len(t, decoded.Extend, 2)
		assert.Equal(t, tx.Extend[0].EntityKey, decoded.Extend[0].EntityKey)
		assert.Equal(t, tx.Extend[0].NumberOfBlocks, decoded.Extend[0].NumberOfBlocks)
		assert.Equal(t, tx.Extend[1].EntityKey, decoded.Extend[1].EntityKey)
		assert.Equal(t, tx.Extend[1].NumberOfBlocks, decoded.Extend[1].NumberOfBlocks)
	})
}

func TestStorageTransactionValidation(t *testing.T) {
	t.Run("ValidTransaction", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Create: []storagetx.Create{
				{
					BTL:     100,
					Payload: []byte("test payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "type", Value: "test"},
						{Key: "name_with_underscore", Value: "example"},
					},
					NumericAnnotations: []entity.NumericAnnotation{
						{Key: "version", Value: 1},
						{Key: "size_bytes", Value: 1024},
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
			Extend: []storagetx.ExtendBTL{
				{
					EntityKey:      common.HexToHash("0xabcdef"),
					NumberOfBlocks: 500,
				},
			},
		}

		err := tx.Validate()
		assert.NoError(t, err)
	})

	t.Run("CreateWithZeroBTL", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Create: []storagetx.Create{
				{
					BTL:     0, // Invalid: BTL cannot be 0
					Payload: []byte("test payload"),
				},
			},
		}

		err := tx.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "create BTL is 0")
	})

	t.Run("UpdateWithZeroBTL", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Update: []storagetx.Update{
				{
					EntityKey: common.HexToHash("0x1234567890"),
					BTL:       0, // Invalid: BTL cannot be 0
					Payload:   []byte("updated payload"),
				},
			},
		}

		err := tx.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update[0] BTL is 0")
	})

	t.Run("ExtendWithZeroBlocks", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Extend: []storagetx.ExtendBTL{
				{
					EntityKey:      common.HexToHash("0x1234567890"),
					NumberOfBlocks: 0, // Invalid: NumberOfBlocks cannot be 0
				},
			},
		}

		err := tx.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "extend[0] number of blocks is 0")
	})

	t.Run("InvalidAnnotationKey", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Create: []storagetx.Create{
				{
					BTL:     100,
					Payload: []byte("test payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "$invalid", Value: "test"}, // Invalid: starts with $
					},
				},
			},
		}

		err := tx.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid annotation identifier")
	})

	t.Run("DuplicateStringAnnotationKey", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Create: []storagetx.Create{
				{
					BTL:     100,
					Payload: []byte("test payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "type", Value: "test1"},
						{Key: "type", Value: "test2"}, // Invalid: duplicate key
					},
				},
			},
		}

		err := tx.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "string annotation key type is duplicated")
	})

	t.Run("DuplicateNumericAnnotationKey", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Create: []storagetx.Create{
				{
					BTL:     100,
					Payload: []byte("test payload"),
					NumericAnnotations: []entity.NumericAnnotation{
						{Key: "version", Value: 1},
						{Key: "version", Value: 2}, // Invalid: duplicate key
					},
				},
			},
		}

		err := tx.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "numeric annotation key version is duplicated")
	})

	t.Run("UpdateWithDuplicateAnnotations", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Update: []storagetx.Update{
				{
					EntityKey: common.HexToHash("0x1234567890"),
					BTL:       200,
					Payload:   []byte("updated payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "status", Value: "active"},
						{Key: "status", Value: "inactive"}, // Invalid: duplicate key
					},
				},
			},
		}

		err := tx.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update[0] string annotation key status is duplicated")
	})

	t.Run("ValidAnnotationKeys", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{
			Create: []storagetx.Create{
				{
					BTL:     100,
					Payload: []byte("test payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "type", Value: "test"},
						{Key: "name_with_underscore", Value: "example"},
						{Key: "αβγ", Value: "unicode"}, // Unicode letters should be valid
						{Key: "_starts_with_underscore", Value: "valid"},
					},
					NumericAnnotations: []entity.NumericAnnotation{
						{Key: "version123", Value: 1},
						{Key: "size_bytes", Value: 1024},
					},
				},
			},
		}

		err := tx.Validate()
		assert.NoError(t, err)
	})

	t.Run("EmptyTransactionIsValid", func(t *testing.T) {
		tx := &storagetx.StorageTransaction{}
		err := tx.Validate()
		assert.NoError(t, err)
	})
}
