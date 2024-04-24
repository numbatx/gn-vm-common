package vmcommon

// MetachainShardId will be used to identify a shard ID as metachain
const MetachainShardId = uint32(0xFFFFFFFF)

// AllShardId will be used to identify that a message is for all shards
const AllShardId = uint32(0xFFFFFFF0)

// NumbatProtectedKeyPrefix is the key prefix which is protected from writing in the trie - only for special builtin functions
const NumbatProtectedKeyPrefix = "NUMBAT"

// DCTKeyIdentifier is the key prefix for dct tokens
const DCTKeyIdentifier = "dct"

// DCTRoleIdentifier is the key prefix for dct role identifier
const DCTRoleIdentifier = "role"

// DCTNFTLatestNonceIdentifier is the key prefix for dct latest nonce identifier
const DCTNFTLatestNonceIdentifier = "nonce"

// BuiltInFunctionSetUserName is the key for the set user name built-in function
const BuiltInFunctionSetUserName = "SetUserName"

// BuiltInFunctionDCTBurn is the key for the Dharitri Core Token (DCT) burn built-in function
const BuiltInFunctionDCTBurn = "DCTBurn"

// BuiltInFunctionDCTNFTCreateRoleTransfer is the key for the Dharitri Core Token (DCT) create role transfer function
const BuiltInFunctionDCTNFTCreateRoleTransfer = "DCTNFTCreateRoleTransfer"

// DCTRoleLocalBurn is the constant string for the local role of burn for DCT tokens
const DCTRoleLocalBurn = "DCTRoleLocalBurn"

// BuiltInFunctionDCTTransfer is the key for the Dharitri Core Token (DCT) transfer built-in function
const BuiltInFunctionDCTTransfer = "DCTTransfer"

// BuiltInFunctionDCTNFTTransfer is the key for the Dharitri Core Token (DCT) NFT transfer built-in function
const BuiltInFunctionDCTNFTTransfer = "DCTNFTTransfer"

// MinLenArgumentsDCTTransfer defines the min length of arguments for the DCT transfer
const MinLenArgumentsDCTTransfer = 2

// MinLenArgumentsDCTNFTTransfer defines the minimum length for dct nft transfer
const MinLenArgumentsDCTNFTTransfer = 4

// MaxLenForDCTIssueMint defines the maximum length in bytes for the issued/minted balance
const MaxLenForDCTIssueMint = 100

// DCTRoleLocalMint is the constant string for the local role of mint for DCT tokens
const DCTRoleLocalMint = "DCTRoleLocalMint"

// DCTRoleNFTCreate is the constant string for the local role of create for DCT NFT tokens
const DCTRoleNFTCreate = "DCTRoleNFTCreate"

// DCTRoleNFTAddQuantity is the constant string for the local role of adding quantity for existing DCT NFT tokens
const DCTRoleNFTAddQuantity = "DCTRoleNFTAddQuantity"

// DCTRoleNFTBurn is the constant string for the local role of burn for DCT NFT tokens
const DCTRoleNFTBurn = "DCTRoleNFTBurn"

// DCTRoleNFTAddURI is the constant string for the local role of adding a URI for DCT NFT tokens
const DCTRoleNFTAddURI = "DCTRoleNFTAddURI"

// DCTRoleNFTUpdateAttributes is the constant string for the local role of updating attributes for DCT NFT tokens
const DCTRoleNFTUpdateAttributes = "DCTRoleNFTUpdateAttributes"

// BuiltInFunctionDCTNFTCreate is the key for the Dharitri Core Token (DCT) NFT create built-in function
const BuiltInFunctionDCTNFTCreate = "DCTNFTCreate"

// BuiltInFunctionDCTNFTAddQuantity is the key for the Dharitri Core Token (DCT) NFT add quantity built-in function
const BuiltInFunctionDCTNFTAddQuantity = "DCTNFTAddQuantity"

// BuiltInFunctionDCTNFTAddURI is the key for the Dharitri Core Token (DCT) NFT add URI built-in function
const BuiltInFunctionDCTNFTAddURI = "DCTNFTAddURI"

// BuiltInFunctionDCTNFTUpdateAttributes is the key for the Dharitri Core Token (DCT) NFT update attributes built-in function
const BuiltInFunctionDCTNFTUpdateAttributes = "DCTNFTUpdateAttributes"

// BuiltInFunctionMultiDCTNFTTransfer is the key for the Dharitri Core Token (DCT) multi transfer built-in function
const BuiltInFunctionMultiDCTNFTTransfer = "MultiDCTNFTTransfer"

// BuiltInFunctionClaimDeveloperRewards is the key for the claim developer rewards built-in function
const BuiltInFunctionClaimDeveloperRewards = "ClaimDeveloperRewards"

// BuiltInFunctionChangeOwnerAddress is the key for the change owner built in function built-in function
const BuiltInFunctionChangeOwnerAddress = "ChangeOwnerAddress"

// BuiltInFunctionSaveKeyValue is the key for the save key value built-in function
const BuiltInFunctionSaveKeyValue = "SaveKeyValue"

// BuiltInFunctionDCTFreeze is the key for the Dharitri Core Token (DCT) freeze built-in function
const BuiltInFunctionDCTFreeze = "DCTFreeze"

// BuiltInFunctionDCTUnFreeze is the key for the Dharitri Core Token (DCT) unfreeze built-in function
const BuiltInFunctionDCTUnFreeze = "DCTUnFreeze"

// BuiltInFunctionDCTWipe is the key for the Dharitri Core Token (DCT) wipe built-in function
const BuiltInFunctionDCTWipe = "DCTWipe"

// BuiltInFunctionDCTPause is the key for the Dharitri Core Token (DCT) pause built-in function
const BuiltInFunctionDCTPause = "DCTPause"

// BuiltInFunctionDCTUnPause is the key for the Dharitri Core Token (DCT) unpause built-in function
const BuiltInFunctionDCTUnPause = "DCTUnPause"

// BuiltInFunctionSetDCTRole is the key for the Dharitri Core Token (DCT) set built-in function
const BuiltInFunctionSetDCTRole = "DCTSetRole"

// BuiltInFunctionUnSetDCTRole is the key for the Dharitri Core Token (DCT) unset built-in function
const BuiltInFunctionUnSetDCTRole = "DCTUnSetRole"

// BuiltInFunctionDCTLocalMint is the key for the Dharitri Core Token (DCT) local mint built-in function
const BuiltInFunctionDCTLocalMint = "DCTLocalMint"

// BuiltInFunctionDCTLocalBurn is the key for the Dharitri Core Token (DCT) local burn built-in function
const BuiltInFunctionDCTLocalBurn = "DCTLocalBurn"

// BuiltInFunctionDCTNFTBurn is the key for the Dharitri Core Token (DCT) NFT burn built-in function
const BuiltInFunctionDCTNFTBurn = "DCTNFTBurn"

// BaseOperationCostString represents the field name for base operation costs
const BaseOperationCostString = "BaseOperationCost"

// BuiltInCostString represents the field name for built in operation costs
const BuiltInCostString = "BuiltInCost"

// DCTType defines the possible types in case of DCT tokens
type DCTType uint32

const (
	// Fungible defines the token type for DCT fungible tokens
	Fungible DCTType = iota
	// NonFungible defines the token type for DCT non fungible tokens
	NonFungible
)

// FungibleDCT defines the string for the token type of fungible DCT
const FungibleDCT = "FungibleDCT"

// NonFungibleDCT defines the string for the token type of non fungible DCT
const NonFungibleDCT = "NonFungibleDCT"

// SemiFungibleDCT defines the string for the token type of semi fungible DCT
const SemiFungibleDCT = "SemiFungibleDCT"

// MaxRoyalty defines 100% as uint32
const MaxRoyalty = uint32(10000)

// DCTSCAddress is the hard-coded address for dct issuing smart contract
var DCTSCAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 255, 255}
