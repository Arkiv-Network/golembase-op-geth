package logs

import "github.com/ethereum/go-ethereum/crypto"

// ArkivEntityCreated is the event signature for entity creation logs.
// Parameters: entityKey (indexed), ownerAddress(indexed), expirationBlock, cost (wei)
var ArkivEntityCreated = crypto.Keccak256Hash([]byte("ArkivEntityCreated(uint256,address,uint256,uint256)"))

// ArkivEntityUpdated is the event signature for entity update logs.
// Parameters: entityKey (indexed), ownerAddress(indexed), oldExpirationBlock, newExpirationBlock, cost (wei)
var ArkivEntityUpdated = crypto.Keccak256Hash([]byte("ArkivEntityUpdated(uint256,address,uint256,uint256,uint256)"))

// ArkivEntityExpired is the event signature for entity expiration logs.
// Parameters: entityKey (indexed), ownerAddress(indexed)
var ArkivEntityExpired = crypto.Keccak256Hash([]byte("ArkivEntityExpired(uint256,address)"))

// ArkivEntityDeleted is the event signature for entity deletion logs.
// Parameters: entityKey (indexed), ownerAddress(indexed)
var ArkivEntityDeleted = crypto.Keccak256Hash([]byte("ArkivEntityDeleted(uint256,address)"))

// ArkivEntityBTLExtended is the event signature for extending BTL of an entity.
// Parameters: entityKey (indexed), ownerAddress(indexed), oldExpirationBlock, newExpirationBlock, cost (wei)
var ArkivEntityBTLExtended = crypto.Keccak256Hash([]byte("ArkivEntityBTLExtended(uint256,address,uint256,uint256,uint256)"))

// ArkivEntityOwnerChanged is the event signature for changing the owner of an entity.
// Parameters: entityKey (indexed), oldOwnerAddress(indexed), newOwnerAddress(indexed)
var ArkivEntityOwnerChanged = crypto.Keccak256Hash([]byte("ArkivEntityOwnerChanged(uint256,address,address)"))
