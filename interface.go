package vmcommon

import (
	"math/big"

	"github.com/numbatx/gn-core/core/closing"
	"github.com/numbatx/gn-core/data"
	"github.com/numbatx/gn-core/data/dct"
)

// FunctionNames (alias) is a map of function names
type FunctionNames = map[string]struct{}

// BlockchainHook is the interface for VM blockchain callbacks
type BlockchainHook interface {
	// NewAddress yields the address of a new SC account, when one such account is created.
	// The result should only depend on the creator address and nonce.
	// Returning an empty address lets the VM decide what the new address should be.
	NewAddress(creatorAddress []byte, creatorNonce uint64, vmType []byte) ([]byte, error)

	// GetStorageData should yield the storage value for a certain account and index.
	// Should return an empty byte array if the key is missing from the account storage,
	// or if account does not exist.
	GetStorageData(accountAddress []byte, index []byte) ([]byte, uint32, error)

	// GetBlockhash returns the hash of the block with the asked nonce if available
	GetBlockhash(nonce uint64) ([]byte, error)

	// LastNonce returns the nonce from from the last committed block
	LastNonce() uint64

	// LastRound returns the round from the last committed block
	LastRound() uint64

	// LastTimeStamp returns the timeStamp from the last committed block
	LastTimeStamp() uint64

	// LastRandomSeed returns the random seed from the last committed block
	LastRandomSeed() []byte

	// LastEpoch returns the epoch from the last committed block
	LastEpoch() uint32

	// GetStateRootHash returns the state root hash from the last committed block
	GetStateRootHash() []byte

	// CurrentNonce returns the nonce from the current block
	CurrentNonce() uint64

	// CurrentRound returns the round from the current block
	CurrentRound() uint64

	// CurrentTimeStamp return the timestamp from the current block
	CurrentTimeStamp() uint64

	// CurrentRandomSeed returns the random seed from the current header
	CurrentRandomSeed() []byte

	// CurrentEpoch returns the current epoch
	CurrentEpoch() uint32

	// ProcessBuiltInFunction will process the builtIn function for the created input
	ProcessBuiltInFunction(input *ContractCallInput) (*VMOutput, error)

	// GetBuiltinFunctionNames returns the names of protocol built-in functions
	GetBuiltinFunctionNames() FunctionNames

	// GetAllState returns the full state of the account, all the key-value saved
	GetAllState(address []byte) (map[string][]byte, error)

	// GetUserAccount returns a user account
	GetUserAccount(address []byte) (UserAccountHandler, error)

	// GetCode returns the code for the given account
	GetCode(UserAccountHandler) []byte

	// GetShardOfAddress returns the shard ID of a given address
	GetShardOfAddress(address []byte) uint32

	// IsSmartContract returns whether the address points to a smart contract
	IsSmartContract(address []byte) bool

	// IsPayable checks weather the provided address can receive MOA or not
	IsPayable(sndAddress []byte, recvAddress []byte) (bool, error)

	// SaveCompiledCode saves to cache and storage the compiled code
	SaveCompiledCode(codeHash []byte, code []byte)

	// GetCompiledCode returns the compiled code if it finds in the cache or storage
	GetCompiledCode(codeHash []byte) (bool, []byte)

	// ClearCompiledCodes clears the cache and storage of compiled codes
	ClearCompiledCodes()

	// GetDCTToken loads the DCT digital token for the given key
	GetDCTToken(address []byte, tokenID []byte, nonce uint64) (*dct.DCToken, error)

	// IsPaused returns true if the tokenID is paused globally
	IsPaused(tokenID []byte) bool

	// IsLimitedTransfer return true if the tokenID has limited transfers
	IsLimitedTransfer(tokenID []byte) bool

	// GetSnapshot gets the number of entries in the journal as a snapshot id
	GetSnapshot() int

	// RevertToSnapshot reverts snaphots up to the specified one
	RevertToSnapshot(snapshot int) error

	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// VMExecutionHandler interface for any Numbat VM endpoint
type VMExecutionHandler interface {
	closing.Closer

	// RunSmartContractCreate computes how a smart contract creation should be performed
	RunSmartContractCreate(input *ContractCreateInput) (*VMOutput, error)

	// RunSmartContractCall computes the result of a smart contract call and how the system must change after the execution
	RunSmartContractCall(input *ContractCallInput) (*VMOutput, error)

	// GasScheduleChange sets a new gas schedule for the VM
	GasScheduleChange(newGasSchedule map[string]map[string]uint64)

	// GetVersion returns the version of the VM instance
	GetVersion() string

	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// CryptoHook interface for VM krypto functions
type CryptoHook interface {
	// Sha256 cryptographic function
	Sha256(data []byte) ([]byte, error)

	// Keccak256 cryptographic function
	Keccak256(data []byte) ([]byte, error)

	// Ripemd160 cryptographic function
	Ripemd160(data []byte) ([]byte, error)

	// Ecrecover calculates the corresponding Ethereum address for the public key which created the given signature
	// https://ewasm.readthedocs.io/en/mkdocs/system_contracts/
	Ecrecover(hash []byte, recoveryID []byte, r []byte, s []byte) ([]byte, error)

	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// UserAccountHandler models a user account, which can journalize account's data with some extra features
// like balance, developer rewards, owner
type UserAccountHandler interface {
	GetCodeMetadata() []byte
	GetCodeHash() []byte
	GetRootHash() []byte
	AccountDataHandler() AccountDataHandler
	AddToBalance(value *big.Int) error
	GetBalance() *big.Int
	ClaimDeveloperRewards([]byte) (*big.Int, error)
	GetDeveloperReward() *big.Int
	ChangeOwnerAddress([]byte, []byte) error
	SetOwnerAddress([]byte)
	GetOwnerAddress() []byte
	SetUserName(userName []byte)
	GetUserName() []byte
	AccountHandler
}

// AccountDataHandler models what how to manipulate data held by a SC account
type AccountDataHandler interface {
	RetrieveValue(key []byte) ([]byte, uint32, error)
	SaveKeyValue(key []byte, value []byte) error
	IsInterfaceNil() bool
}

// AccountHandler models a state account, which can journalize and revert
// It knows about code and data, as data structures not hashes
type AccountHandler interface {
	AddressBytes() []byte
	IncreaseNonce(nonce uint64)
	GetNonce() uint64
	IsInterfaceNil() bool
}

// Marshalizer defines the 2 basic operations: serialize (marshal) and deserialize (unmarshal)
type Marshalizer interface {
	Marshal(obj interface{}) ([]byte, error)
	Unmarshal(obj interface{}, buff []byte) error
	IsInterfaceNil() bool
}

// DCTGlobalSettingsHandler provides global settings functions for an DCT token
type DCTGlobalSettingsHandler interface {
	IsPaused(dctTokenKey []byte) bool
	IsLimitedTransfer(dctTokenKey []byte) bool
	IsInterfaceNil() bool
}

// ExtendedDCTGlobalSettingsHandler provides global settings functions for an DCT token
type ExtendedDCTGlobalSettingsHandler interface {
	IsPaused(dctTokenKey []byte) bool
	IsLimitedTransfer(dctTokenKey []byte) bool
	IsBurnForAll(dctTokenKey []byte) bool
	IsSenderOrDestinationWithTransferRole(sender, destination, tokenID []byte) bool
	IsInterfaceNil() bool
}

// DCTRoleHandler provides IsAllowedToExecute function for an DCT
type DCTRoleHandler interface {
	CheckAllowedToExecute(account UserAccountHandler, tokenID []byte, action []byte) error
	IsInterfaceNil() bool
}

// PayableHandler provides IsPayable function which returns if an account is payable or not
type PayableHandler interface {
	IsPayable(sndAddress, rcvAddress []byte) (bool, error)
	IsInterfaceNil() bool
}

// Coordinator defines what a shard state coordinator should hold
type Coordinator interface {
	NumberOfShards() uint32
	ComputeId(address []byte) uint32
	SelfId() uint32
	SameShard(firstAddress, secondAddress []byte) bool
	CommunicationIdentifier(destShardID uint32) string
	IsInterfaceNil() bool
}

// AccountsAdapter is used for the structure that manages the accounts on top of a trie.PatriciaMerkleTrie
// implementation
type AccountsAdapter interface {
	GetExistingAccount(address []byte) (AccountHandler, error)
	LoadAccount(address []byte) (AccountHandler, error)
	SaveAccount(account AccountHandler) error
	RemoveAccount(address []byte) error
	Commit() ([]byte, error)
	JournalLen() int
	RevertToSnapshot(snapshot int) error
	GetCode(codeHash []byte) []byte

	RootHash() ([]byte, error)
	IsInterfaceNil() bool
}

// BuiltinFunction defines the methods for the built-in protocol smart contract functions
type BuiltinFunction interface {
	ProcessBuiltinFunction(acntSnd, acntDst UserAccountHandler, vmInput *ContractCallInput) (*VMOutput, error)
	SetNewGasConfig(gasCost *GasCost)
	IsActive() bool
	IsInterfaceNil() bool
}

// BuiltInFunctionContainer defines the methods for the built-in protocol container
type BuiltInFunctionContainer interface {
	Get(key string) (BuiltinFunction, error)
	Add(key string, function BuiltinFunction) error
	Replace(key string, function BuiltinFunction) error
	Remove(key string)
	Len() int
	Keys() map[string]struct{}
	IsInterfaceNil() bool
}

// EpochSubscriberHandler defines the behavior of a component that can be notified if a new epoch was confirmed
type EpochSubscriberHandler interface {
	EpochConfirmed(epoch uint32, timestamp uint64)
	IsInterfaceNil() bool
}

// EpochNotifier can notify upon an epoch change and provide the current epoch
type EpochNotifier interface {
	RegisterNotifyHandler(handler EpochSubscriberHandler)
	IsInterfaceNil() bool
}

// DCTTransferParser can parse single and multi DCT / NFT transfers
type DCTTransferParser interface {
	ParseDCTTransfers(sndAddr []byte, rcvAddr []byte, function string, args [][]byte) (*ParsedDCTTransfers, error)
	IsInterfaceNil() bool
}

// DCTNFTStorageHandler will handle the storage for the nft metadata
type DCTNFTStorageHandler interface {
	SaveDCTNFTToken(senderAddress []byte, acnt UserAccountHandler, dctTokenKey []byte, nonce uint64, dctData *dct.DCToken, isCreation bool, isReturnWithError bool) ([]byte, error)
	GetDCTNFTTokenOnSender(acnt UserAccountHandler, dctTokenKey []byte, nonce uint64) (*dct.DCToken, error)
	GetDCTNFTTokenOnDestination(acnt UserAccountHandler, dctTokenKey []byte, nonce uint64) (*dct.DCToken, bool, error)
	GetDCTNFTTokenOnDestinationWithCustomSystemAccount(accnt UserAccountHandler, dctTokenKey []byte, nonce uint64, systemAccount UserAccountHandler) (*dct.DCToken, bool, error)
	WasAlreadySentToDestinationShardAndUpdateState(tickerID []byte, nonce uint64, dstAddress []byte) (bool, error)
	SaveNFTMetaDataToSystemAccount(tx data.TransactionHandler) error
	AddToLiquiditySystemAcc(dctTokenKey []byte, nonce uint64, transferValue *big.Int) error
	IsInterfaceNil() bool
}

// SimpleDCTNFTStorageHandler will handle get of DCT data and save metadata to system acc
type SimpleDCTNFTStorageHandler interface {
	GetDCTNFTTokenOnDestination(accnt UserAccountHandler, dctTokenKey []byte, nonce uint64) (*dct.DCToken, bool, error)
	SaveNFTMetaDataToSystemAccount(tx data.TransactionHandler) error
	IsInterfaceNil() bool
}

// CallArgsParser will handle parsing transaction data to function and arguments
type CallArgsParser interface {
	ParseData(data string) (string, [][]byte, error)
	ParseArguments(data string) ([][]byte, error)
	IsInterfaceNil() bool
}

// BuiltInFunctionFactory will handle built-in functions and components
type BuiltInFunctionFactory interface {
	DCTGlobalSettingsHandler() DCTGlobalSettingsHandler
	NFTStorageHandler() SimpleDCTNFTStorageHandler
	BuiltInFunctionContainer() BuiltInFunctionContainer
	SetPayableHandler(handler PayableHandler) error
	CreateBuiltInFunctionContainer() error
	IsInterfaceNil() bool
}

// PayableChecker will handle checking if transfer can happen of DCT tokens towards destination
type PayableChecker interface {
	CheckPayable(vmInput *ContractCallInput, dstAddress []byte, minLenArguments int) error
	DetermineIsSCCallAfter(vmInput *ContractCallInput, destAddress []byte, minLenArguments int) bool
	IsInterfaceNil() bool
}

// AcceptPayableChecker defines the methods to accept a payable handler through a set function
type AcceptPayableChecker interface {
	SetPayableChecker(payableHandler PayableChecker) error
	IsInterfaceNil() bool
}

// EnableEpochsHandler is used to verify which flags are set in the current epoch based on EnableEpochs config
type EnableEpochsHandler interface {
	IsGlobalMintBurnFlagEnabled() bool
	IsDCTTransferRoleFlagEnabled() bool
	IsBuiltInFunctionsFlagEnabled() bool
	IsCheckCorrectTokenIDForTransferRoleFlagEnabled() bool
	IsMultiDCTTransferFixOnCallBackFlagEnabled() bool
	IsFixOOGReturnCodeFlagEnabled() bool
	IsRemoveNonUpdatedStorageFlagEnabled() bool
	IsCreateNFTThroughExecByCallerFlagEnabled() bool
	IsStorageAPICostOptimizationFlagEnabled() bool
	IsFailExecutionOnEveryAPIErrorFlagEnabled() bool
	IsManagedCryptoAPIsFlagEnabled() bool
	IsSCDeployFlagEnabled() bool
	IsAheadOfTimeGasUsageFlagEnabled() bool
	IsRepairCallbackFlagEnabled() bool
	IsDisableExecByCallerFlagEnabled() bool
	IsRefactorContextFlagEnabled() bool
	IsCheckFunctionArgumentFlagEnabled() bool
	IsCheckExecuteOnReadOnlyFlagEnabled() bool
	IsFixAsyncCallbackCheckFlagEnabled() bool
	IsSaveToSystemAccountFlagEnabled() bool
	IsCheckFrozenCollectionFlagEnabled() bool
	IsSendAlwaysFlagEnabled() bool
	IsValueLengthCheckFlagEnabled() bool
	IsCheckTransferFlagEnabled() bool
	IsTransferToMetaFlagEnabled() bool
	IsDCTNFTImprovementV1FlagEnabled() bool
	IsFixOldTokenLiquidityEnabled() bool
	IsRuntimeMemStoreLimitEnabled() bool
	IsWipeSingleNFTLiquidityDecreaseEnabled() bool

	MultiDCTTransferAsyncCallBackEnableEpoch() uint32
	FixOOGReturnCodeEnableEpoch() uint32
	RemoveNonUpdatedStorageEnableEpoch() uint32
	CreateNFTThroughExecByCallerEnableEpoch() uint32
	FixFailExecutionOnErrorEnableEpoch() uint32
	ManagedCryptoAPIEnableEpoch() uint32
	DisableExecByCallerEnableEpoch() uint32
	RefactorContextEnableEpoch() uint32
	CheckExecuteReadOnlyEnableEpoch() uint32
	StorageAPICostOptimizationEnableEpoch() uint32

	IsInterfaceNil() bool
}
