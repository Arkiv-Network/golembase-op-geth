package stateblob

import (
	"encoding/binary"
	"iter"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/holiman/uint256"
)

type StateAccess = storageutil.StateAccess

func SetBlob(db StateAccess, key common.Hash, value []byte) {
	keyInt := new(uint256.Int).SetBytes(key[:])

	for v := range BytesTo32ByteSequence(value) {
		db.SetState(address.ArkivProcessorAddress, keyInt.Bytes32(), v)
		keyInt.AddUint64(keyInt, 1)
	}
}

func BytesTo32ByteSequence(value []byte) iter.Seq[common.Hash] {
	return func(yield func(common.Hash) bool) {
		// For small values that fit in a single hash with length byte
		if len(value) <= 31 {
			// Create a 32-byte array with the data and length
			data := common.RightPadBytes(value, 32)
			// Set the length in the last byte (2*n for small payloads)
			data[31] = byte(len(value) * 2)
			yield(common.BytesToHash(data))
			return
		}

		// For large values, first chunk contains the length
		// Use the length 2*n+1 for larger payloads
		length := uint256.NewInt(uint64(len(value)*2 + 1))
		if !yield(common.BytesToHash(length.Bytes())) {
			return
		}

		// Subsequent chunks contain the actual data
		for start := 0; start < len(value); start += 32 {
			end := start + 32
			if end > len(value) {
				end = len(value)
			}
			chunk := common.RightPadBytes(value[start:end], 32)
			if !yield(common.BytesToHash(chunk)) {
				return
			}
		}
	}
}

func GetBlob(db StateAccess, key common.Hash) []byte {
	head := db.GetState(address.ArkivProcessorAddress, key)
	if head == emptyHash {
		return []byte{}
	}

	// Check if this is a small payload (last bit set)
	if head[31]&0x01 == 0 {
		// For small payloads, length is stored in the last byte (2*n+1)
		length := head[31] / 2
		return head[:length]
	}

	// For large payloads
	keyInt := new(uint256.Int).SetBytes(key[:])

	// First chunk contains the length
	length := binary.BigEndian.Uint64(head[24:])
	dataLength := (length - 1) / 2 // Subtract 1 to account for the length marker

	value := make([]byte, 0, dataLength)
	remaining := dataLength

	// Skip the length chunk
	keyInt.AddUint64(keyInt, 1)

	// Read data chunks
	for remaining > 0 {
		chunk := db.GetState(address.ArkivProcessorAddress, keyInt.Bytes32())
		size := min(remaining, 32)
		value = append(value, chunk[:size]...)
		remaining -= size
		keyInt.AddUint64(keyInt, 1)
	}

	return value
}

var emptyHash = common.Hash{}

func DeleteBlob(db StateAccess, key common.Hash) {
	head := db.GetState(address.ArkivProcessorAddress, key)
	if head == emptyHash {
		return
	}

	// Clear the head slot
	db.SetState(address.ArkivProcessorAddress, key, emptyHash)

	// For small payloads (≤31 bytes), we only need to clear the head slot
	if head[31]&0x01 == 0 {
		return
	}

	// For large payloads
	length := binary.BigEndian.Uint64(head[24:])
	dataLength := (length - 1) / 2 // Subtract 1 to account for the length marker
	numberOfSlots := (dataLength + 31) / 32

	keyInt := new(uint256.Int).SetBytes(key[:])

	// Clear all data slots (skip the length slot which was already cleared)
	keyInt.AddUint64(keyInt, 1)
	for range numberOfSlots {
		db.SetState(address.ArkivProcessorAddress, keyInt.Bytes32(), emptyHash)
		keyInt.AddUint64(keyInt, 1)
	}
}
