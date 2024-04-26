package builtInFunctions

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-core/data"
	"github.com/numbatx/gn-core/data/dct"
	vmcommon "github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/atomic"
	"github.com/numbatx/gn-vm-common/parsers"
)

const existsOnShard = byte(1)

type dctDataStorage struct {
	accounts                vmcommon.AccountsAdapter
	globalSettingsHandler   vmcommon.DCTGlobalSettingsHandler
	marshalizer             vmcommon.Marshalizer
	keyPrefix               []byte
	flagSaveToSystemAccount atomic.Flag
	saveToSystemEnableEpoch uint32
	shardCoordinator        vmcommon.Coordinator
	txDataParser            vmcommon.CallArgsParser
}

// ArgsNewDCTDataStorage defines the argument list for new dct data storage handler
type ArgsNewDCTDataStorage struct {
	Accounts                vmcommon.AccountsAdapter
	GlobalSettingsHandler   vmcommon.DCTGlobalSettingsHandler
	Marshalizer             vmcommon.Marshalizer
	SaveToSystemEnableEpoch uint32
	EpochNotifier           vmcommon.EpochNotifier
	ShardCoordinator        vmcommon.Coordinator
}

// NewDCTDataStorage creates a new dct data storage handler
func NewDCTDataStorage(args ArgsNewDCTDataStorage) (*dctDataStorage, error) {
	if check.IfNil(args.Accounts) {
		return nil, ErrNilAccountsAdapter
	}
	if check.IfNil(args.GlobalSettingsHandler) {
		return nil, ErrNilGlobalSettingsHandler
	}
	if check.IfNil(args.Marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(args.EpochNotifier) {
		return nil, ErrNilEpochHandler
	}
	if check.IfNil(args.ShardCoordinator) {
		return nil, ErrNilShardCoordinator
	}

	e := &dctDataStorage{
		accounts:                args.Accounts,
		globalSettingsHandler:   args.GlobalSettingsHandler,
		marshalizer:             args.Marshalizer,
		keyPrefix:               []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier),
		flagSaveToSystemAccount: atomic.Flag{},
		saveToSystemEnableEpoch: args.SaveToSystemEnableEpoch,
		shardCoordinator:        args.ShardCoordinator,
		txDataParser:            parsers.NewCallArgsParser(),
	}

	args.EpochNotifier.RegisterNotifyHandler(e)

	return e, nil
}

// GetDCTNFTTokenOnSender gets the nft token on sender account
func (e *dctDataStorage) GetDCTNFTTokenOnSender(
	accnt vmcommon.UserAccountHandler,
	dctTokenKey []byte,
	nonce uint64,
) (*dct.DCToken, error) {
	dctData, isNew, err := e.GetDCTNFTTokenOnDestination(accnt, dctTokenKey, nonce)
	if err != nil {
		return nil, err
	}
	if isNew {
		return nil, ErrNewNFTDataOnSenderAddress
	}

	return dctData, nil
}

// GetDCTNFTTokenOnDestination gets the nft token on destination account
func (e *dctDataStorage) GetDCTNFTTokenOnDestination(
	accnt vmcommon.UserAccountHandler,
	dctTokenKey []byte,
	nonce uint64,
) (*dct.DCToken, bool, error) {
	dctNFTTokenKey := computeDCTNFTTokenKey(dctTokenKey, nonce)
	dctData := &dct.DCToken{
		Value: big.NewInt(0),
		Type:  uint32(core.Fungible),
	}
	marshaledData, err := accnt.AccountDataHandler().RetrieveValue(dctNFTTokenKey)
	if err != nil || len(marshaledData) == 0 {
		return dctData, true, nil
	}

	err = e.marshalizer.Unmarshal(dctData, marshaledData)
	if err != nil {
		return nil, false, err
	}

	if !e.flagSaveToSystemAccount.IsSet() || nonce == 0 {
		return dctData, false, nil
	}

	dctMetaData, err := e.getDCTMetaDataFromSystemAccount(dctNFTTokenKey)
	if err != nil {
		return nil, false, err
	}
	if dctMetaData != nil {
		dctData.TokenMetaData = dctMetaData
	}

	return dctData, false, nil
}

func (e *dctDataStorage) getDCTDigitalTokenDataFromSystemAccount(
	tokenKey []byte,
) (*dct.DCToken, vmcommon.UserAccountHandler, error) {
	systemAcc, err := e.getSystemAccount()
	if err != nil {
		return nil, nil, err
	}

	marshaledData, err := systemAcc.AccountDataHandler().RetrieveValue(tokenKey)
	if err != nil || len(marshaledData) == 0 {
		return nil, systemAcc, nil
	}

	dctData := &dct.DCToken{}
	err = e.marshalizer.Unmarshal(dctData, marshaledData)
	if err != nil {
		return nil, nil, err
	}

	return dctData, systemAcc, nil
}

func (e *dctDataStorage) getDCTMetaDataFromSystemAccount(
	tokenKey []byte,
) (*dct.MetaData, error) {
	dctData, _, err := e.getDCTDigitalTokenDataFromSystemAccount(tokenKey)
	if err != nil {
		return nil, err
	}
	if dctData == nil {
		return nil, nil
	}

	return dctData.TokenMetaData, nil
}

// SaveDCTNFTToken saves the nft token to the account and system account
func (e *dctDataStorage) SaveDCTNFTToken(
	senderAddress []byte,
	acnt vmcommon.UserAccountHandler,
	dctTokenKey []byte,
	nonce uint64,
	dctData *dct.DCToken,
	mustUpdate bool,
	isReturnWithError bool,
) ([]byte, error) {
	err := checkFrozeAndPause(acnt.AddressBytes(), dctTokenKey, dctData, e.globalSettingsHandler, isReturnWithError)
	if err != nil {
		return nil, err
	}

	dctNFTTokenKey := computeDCTNFTTokenKey(dctTokenKey, nonce)
	err = checkFrozeAndPause(acnt.AddressBytes(), dctNFTTokenKey, dctData, e.globalSettingsHandler, isReturnWithError)
	if err != nil {
		return nil, err
	}

	if dctData.Value.Cmp(zero) <= 0 {
		return nil, acnt.AccountDataHandler().SaveKeyValue(dctNFTTokenKey, nil)
	}

	if !e.flagSaveToSystemAccount.IsSet() {
		marshaledData, err := e.marshalizer.Marshal(dctData)
		if err != nil {
			return nil, err
		}

		return marshaledData, acnt.AccountDataHandler().SaveKeyValue(dctNFTTokenKey, marshaledData)
	}

	senderShardID := e.shardCoordinator.ComputeId(senderAddress)
	err = e.saveDCTMetaDataToSystemAccount(senderShardID, dctNFTTokenKey, nonce, dctData, mustUpdate)
	if err != nil {
		return nil, err
	}

	dctDataOnAccount := &dct.DCToken{
		Type:       dctData.Type,
		Value:      dctData.Value,
		Properties: dctData.Properties,
	}
	marshaledData, err := e.marshalizer.Marshal(dctDataOnAccount)
	if err != nil {
		return nil, err
	}

	return marshaledData, acnt.AccountDataHandler().SaveKeyValue(dctNFTTokenKey, marshaledData)
}

func (e *dctDataStorage) saveDCTMetaDataToSystemAccount(
	senderShardID uint32,
	dctNFTTokenKey []byte,
	nonce uint64,
	dctData *dct.DCToken,
	mustUpdate bool,
) error {
	if nonce == 0 {
		return nil
	}
	if dctData.TokenMetaData == nil {
		return nil
	}

	systemAcc, err := e.getSystemAccount()
	if err != nil {
		return err
	}

	currentSaveData, err := systemAcc.AccountDataHandler().RetrieveValue(dctNFTTokenKey)
	if !mustUpdate && len(currentSaveData) > 0 {
		return nil
	}

	dctDataOnSystemAcc := &dct.DCToken{
		Type:          dctData.Type,
		Value:         big.NewInt(0),
		TokenMetaData: dctData.TokenMetaData,
		Properties:    make([]byte, e.shardCoordinator.NumberOfShards()),
	}
	selfID := e.shardCoordinator.SelfId()
	if selfID != core.MetachainShardId {
		dctDataOnSystemAcc.Properties[selfID] = existsOnShard
	}
	if senderShardID != core.MetachainShardId {
		dctDataOnSystemAcc.Properties[senderShardID] = existsOnShard
	}

	return e.marshalAndSaveData(systemAcc, dctDataOnSystemAcc, dctNFTTokenKey)
}

func (e *dctDataStorage) marshalAndSaveData(
	systemAcc vmcommon.UserAccountHandler,
	dctData *dct.DCToken,
	dctNFTTokenKey []byte,
) error {
	marshaledData, err := e.marshalizer.Marshal(dctData)
	if err != nil {
		return err
	}

	err = systemAcc.AccountDataHandler().SaveKeyValue(dctNFTTokenKey, marshaledData)
	if err != nil {
		return err
	}

	return e.accounts.SaveAccount(systemAcc)
}

func (e *dctDataStorage) getSystemAccount() (vmcommon.UserAccountHandler, error) {
	systemSCAccount, err := e.accounts.LoadAccount(vmcommon.SystemAccountAddress)
	if err != nil {
		return nil, err
	}

	userAcc, ok := systemSCAccount.(vmcommon.UserAccountHandler)
	if !ok {
		return nil, ErrWrongTypeAssertion
	}

	return userAcc, nil
}

//TODO: merge properties in case of shard merge

// WasAlreadySentToDestinationShardAndUpdateState checks whether NFT metadata was sent to destination shard or not
// and saves the destination shard as sent
func (e *dctDataStorage) WasAlreadySentToDestinationShardAndUpdateState(
	tickerID []byte,
	nonce uint64,
	dstAddress []byte,
) (bool, error) {
	if !e.flagSaveToSystemAccount.IsSet() {
		return false, nil
	}
	if nonce == 0 {
		return true, nil
	}
	dstShardID := e.shardCoordinator.ComputeId(dstAddress)
	if dstShardID == e.shardCoordinator.SelfId() {
		return true, nil
	}
	if dstShardID == core.MetachainShardId {
		return true, nil
	}

	dctTokenKey := append(e.keyPrefix, tickerID...)
	dctNFTTokenKey := computeDCTNFTTokenKey(dctTokenKey, nonce)

	dctData, systemAcc, err := e.getDCTDigitalTokenDataFromSystemAccount(dctNFTTokenKey)
	if err != nil {
		return false, err
	}
	if dctData == nil {
		return false, nil
	}

	if uint32(len(dctData.Properties)) < e.shardCoordinator.NumberOfShards() {
		newSlice := make([]byte, e.shardCoordinator.NumberOfShards())
		for i, val := range dctData.Properties {
			newSlice[i] = val
		}
		dctData.Properties = newSlice
	}

	if dctData.Properties[dstShardID] > 0 {
		return true, nil
	}

	dctData.Properties[dstShardID] = existsOnShard
	return false, e.marshalAndSaveData(systemAcc, dctData, dctNFTTokenKey)
}

// SaveNFTMetaDataToSystemAccount this saves the NFT metadata to the system account even if there was an error in processing
func (e *dctDataStorage) SaveNFTMetaDataToSystemAccount(
	tx data.TransactionHandler,
) error {
	if !e.flagSaveToSystemAccount.IsSet() {
		return nil
	}

	if check.IfNil(tx) {
		return ErrNilTransactionHandler
	}

	sndShardID := e.shardCoordinator.ComputeId(tx.GetSndAddr())
	dstShardID := e.shardCoordinator.ComputeId(tx.GetRcvAddr())
	isCrossShardTxAtDest := sndShardID != dstShardID && e.shardCoordinator.SelfId() == dstShardID
	if !isCrossShardTxAtDest {
		return nil
	}

	function, arguments, err := e.txDataParser.ParseData(string(tx.GetData()))
	if err != nil {
		return nil
	}
	if len(arguments) < 4 {
		return nil
	}

	switch function {
	case core.BuiltInFunctionDCTNFTTransfer:
		return e.addMetaDataToSystemAccountFromNFTTransfer(sndShardID, arguments)
	case core.BuiltInFunctionMultiDCTNFTTransfer:
		return e.addMetaDataToSystemAccountFromMultiTransfer(sndShardID, arguments)
	default:
		return nil
	}
}

func (e *dctDataStorage) addMetaDataToSystemAccountFromNFTTransfer(
	sndShardID uint32,
	arguments [][]byte,
) error {
	if !bytes.Equal(arguments[3], zeroByteArray) {
		dctTransferData := &dct.DCToken{}
		err := e.marshalizer.Unmarshal(dctTransferData, arguments[3])
		if err != nil {
			return err
		}
		dctTokenKey := append(e.keyPrefix, arguments[0]...)
		nonce := big.NewInt(0).SetBytes(arguments[1]).Uint64()
		dctNFTTokenKey := computeDCTNFTTokenKey(dctTokenKey, nonce)

		return e.saveDCTMetaDataToSystemAccount(sndShardID, dctNFTTokenKey, nonce, dctTransferData, true)
	}
	return nil
}

func (e *dctDataStorage) addMetaDataToSystemAccountFromMultiTransfer(
	sndShardID uint32,
	arguments [][]byte,
) error {
	numOfTransfers := big.NewInt(0).SetBytes(arguments[0]).Uint64()
	if numOfTransfers == 0 {
		return fmt.Errorf("%w, 0 tokens to transfer", ErrInvalidArguments)
	}
	minNumOfArguments := numOfTransfers*argumentsPerTransfer + 1
	if uint64(len(arguments)) < minNumOfArguments {
		return fmt.Errorf("%w, invalid number of arguments", ErrInvalidArguments)
	}

	startIndex := uint64(1)
	for i := uint64(0); i < numOfTransfers; i++ {
		tokenStartIndex := startIndex + i*argumentsPerTransfer
		tokenID := arguments[tokenStartIndex]
		nonce := big.NewInt(0).SetBytes(arguments[tokenStartIndex+1]).Uint64()

		if nonce > 0 && len(arguments[tokenStartIndex+2]) > vmcommon.MaxLengthForValueToOptTransfer {
			dctTransferData := &dct.DCToken{}
			marshaledNFTTransfer := arguments[tokenStartIndex+2]
			err := e.marshalizer.Unmarshal(dctTransferData, marshaledNFTTransfer)
			if err != nil {
				return fmt.Errorf("%w for token %s", err, string(tokenID))
			}

			dctTokenKey := append(e.keyPrefix, tokenID...)
			dctNFTTokenKey := computeDCTNFTTokenKey(dctTokenKey, nonce)
			err = e.saveDCTMetaDataToSystemAccount(sndShardID, dctNFTTokenKey, nonce, dctTransferData, true)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (e *dctDataStorage) EpochConfirmed(epoch uint32, _ uint64) {
	e.flagSaveToSystemAccount.Toggle(epoch >= e.saveToSystemEnableEpoch)
	log.Debug("DCT NFT save to system account", "enabled", e.flagSaveToSystemAccount.IsSet())
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctDataStorage) IsInterfaceNil() bool {
	return e == nil
}
