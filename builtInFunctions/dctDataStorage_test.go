package builtInFunctions

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/big"
	"testing"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-core/data/smartContractResult"
	vmcommon "github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/mock"
	"github.com/stretchr/testify/assert"
)

func createNewDCTDataStorageHandler() *dctDataStorage {
	acnt := mock.NewUserAccount(vmcommon.SystemAccountAddress)
	accounts := &mock.AccountsStub{LoadAccountCalled: func(address []byte) (vmcommon.AccountHandler, error) {
		return acnt, nil
	}}
	args := ArgsNewDCTDataStorage{
		Accounts:                accounts,
		GlobalSettingsHandler:   &mock.GlobalSettingsHandlerStub{},
		Marshalizer:             &mock.MarshalizerMock{},
		SaveToSystemEnableEpoch: 0,
		EpochNotifier:           &mock.EpochNotifierStub{},
		ShardCoordinator:        &mock.ShardCoordinatorStub{},
	}
	dataStore, _ := NewDCTDataStorage(args)
	return dataStore
}

func createMockArgsForNewDCTDataStorage() ArgsNewDCTDataStorage {
	acnt := mock.NewUserAccount(vmcommon.SystemAccountAddress)
	accounts := &mock.AccountsStub{LoadAccountCalled: func(address []byte) (vmcommon.AccountHandler, error) {
		return acnt, nil
	}}
	args := ArgsNewDCTDataStorage{
		Accounts:                accounts,
		GlobalSettingsHandler:   &mock.GlobalSettingsHandlerStub{},
		Marshalizer:             &mock.MarshalizerMock{},
		SaveToSystemEnableEpoch: 0,
		EpochNotifier:           &mock.EpochNotifierStub{},
		ShardCoordinator:        &mock.ShardCoordinatorStub{},
	}
	return args
}

func createNewDCTDataStorageHandlerWithArgs(
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	accounts vmcommon.AccountsAdapter,
) *dctDataStorage {
	args := ArgsNewDCTDataStorage{
		Accounts:                accounts,
		GlobalSettingsHandler:   globalSettingsHandler,
		Marshalizer:             &mock.MarshalizerMock{},
		SaveToSystemEnableEpoch: 10,
		EpochNotifier:           &mock.EpochNotifierStub{},
		ShardCoordinator:        &mock.ShardCoordinatorStub{},
	}
	dataStore, _ := NewDCTDataStorage(args)
	return dataStore
}

func TestNewDCTDataStorage(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	args.Marshalizer = nil
	e, err := NewDCTDataStorage(args)
	assert.Nil(t, e)
	assert.Equal(t, err, ErrNilMarshalizer)

	args = createMockArgsForNewDCTDataStorage()
	args.Accounts = nil
	e, err = NewDCTDataStorage(args)
	assert.Nil(t, e)
	assert.Equal(t, err, ErrNilAccountsAdapter)

	args = createMockArgsForNewDCTDataStorage()
	args.ShardCoordinator = nil
	e, err = NewDCTDataStorage(args)
	assert.Nil(t, e)
	assert.Equal(t, err, ErrNilShardCoordinator)

	args = createMockArgsForNewDCTDataStorage()
	args.GlobalSettingsHandler = nil
	e, err = NewDCTDataStorage(args)
	assert.Nil(t, e)
	assert.Equal(t, err, ErrNilGlobalSettingsHandler)

	args = createMockArgsForNewDCTDataStorage()
	args.EpochNotifier = nil
	e, err = NewDCTDataStorage(args)
	assert.Nil(t, e)
	assert.Equal(t, err, ErrNilEpochHandler)

	args = createMockArgsForNewDCTDataStorage()
	e, err = NewDCTDataStorage(args)
	assert.Nil(t, err)
	assert.False(t, e.IsInterfaceNil())
}

func TestDctDataStorage_GetDCTNFTTokenOnDestinationNoDataInSystemAcc(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	e, _ := NewDCTDataStorage(args)

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{
		TokenMetaData: &dct.MetaData{
			Name: []byte("test"),
		},
		Value: big.NewInt(10),
	}

	tokenIdentifier := "testTkn"
	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + tokenIdentifier
	nonce := uint64(10)
	dctDataBytes, _ := args.Marshalizer.Marshal(dctData)
	tokenKey := append([]byte(key), big.NewInt(int64(nonce)).Bytes()...)
	_ = userAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctDataBytes)

	dctDataGet, _, err := e.GetDCTNFTTokenOnDestination(userAcc, []byte(key), nonce)
	assert.Nil(t, err)
	assert.Equal(t, dctData, dctDataGet)
}

func TestDctDataStorage_GetDCTNFTTokenOnDestinationGetDataFromSystemAcc(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	e, _ := NewDCTDataStorage(args)

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{
		Value: big.NewInt(10),
	}

	tokenIdentifier := "testTkn"
	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + tokenIdentifier
	nonce := uint64(10)
	dctDataBytes, _ := args.Marshalizer.Marshal(dctData)
	tokenKey := append([]byte(key), big.NewInt(int64(nonce)).Bytes()...)
	_ = userAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctDataBytes)

	systemAcc, _ := e.getSystemAccount()
	metaData := &dct.MetaData{
		Name: []byte("test"),
	}
	dctDataOnSystemAcc := &dct.DCToken{TokenMetaData: metaData}
	dctMetaDataBytes, _ := args.Marshalizer.Marshal(dctDataOnSystemAcc)
	_ = systemAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctMetaDataBytes)

	dctDataGet, _, err := e.GetDCTNFTTokenOnDestination(userAcc, []byte(key), nonce)
	assert.Nil(t, err)
	dctData.TokenMetaData = metaData
	assert.Equal(t, dctData, dctDataGet)
}

func TestDctDataStorage_GetDCTNFTTokenOnDestinationMarshalERR(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	e, _ := NewDCTDataStorage(args)

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{
		Value: big.NewInt(10),
		TokenMetaData: &dct.MetaData{
			Name: []byte("test"),
		},
	}

	tokenIdentifier := "testTkn"
	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + tokenIdentifier
	nonce := uint64(10)
	dctDataBytes, _ := args.Marshalizer.Marshal(dctData)
	dctDataBytes = append(dctDataBytes, dctDataBytes...)
	tokenKey := append([]byte(key), big.NewInt(int64(nonce)).Bytes()...)
	_ = userAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctDataBytes)

	_, _, err := e.GetDCTNFTTokenOnDestination(userAcc, []byte(key), nonce)
	assert.NotNil(t, err)

	_, err = e.GetDCTNFTTokenOnSender(userAcc, []byte(key), nonce)
	assert.NotNil(t, err)
}

func TestDctDataStorage_MarshalErrorOnSystemACC(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	e, _ := NewDCTDataStorage(args)

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{
		Value: big.NewInt(10),
	}

	tokenIdentifier := "testTkn"
	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + tokenIdentifier
	nonce := uint64(10)
	dctDataBytes, _ := args.Marshalizer.Marshal(dctData)
	tokenKey := append([]byte(key), big.NewInt(int64(nonce)).Bytes()...)
	_ = userAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctDataBytes)

	systemAcc, _ := e.getSystemAccount()
	metaData := &dct.MetaData{
		Name: []byte("test"),
	}
	dctDataOnSystemAcc := &dct.DCToken{TokenMetaData: metaData}
	dctMetaDataBytes, _ := args.Marshalizer.Marshal(dctDataOnSystemAcc)
	dctMetaDataBytes = append(dctMetaDataBytes, dctMetaDataBytes...)
	_ = systemAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctMetaDataBytes)

	_, _, err := e.GetDCTNFTTokenOnDestination(userAcc, []byte(key), nonce)
	assert.NotNil(t, err)
}

func TestDCTDataStorage_saveDataToSystemAccNotNFTOrMetaData(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	e, _ := NewDCTDataStorage(args)

	err := e.saveDCTMetaDataToSystemAccount(0, []byte("TCK"), 0, nil, true)
	assert.Nil(t, err)

	err = e.saveDCTMetaDataToSystemAccount(0, []byte("TCK"), 1, &dct.DCToken{}, true)
	assert.Nil(t, err)
}

func TestDctDataStorage_SaveDCTNFTTokenNoChangeInSystemAcc(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	e, _ := NewDCTDataStorage(args)

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{
		Value: big.NewInt(10),
	}

	tokenIdentifier := "testTkn"
	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + tokenIdentifier
	nonce := uint64(10)
	dctDataBytes, _ := args.Marshalizer.Marshal(dctData)
	tokenKey := append([]byte(key), big.NewInt(int64(nonce)).Bytes()...)
	_ = userAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctDataBytes)

	systemAcc, _ := e.getSystemAccount()
	metaData := &dct.MetaData{
		Name: []byte("test"),
	}
	dctDataOnSystemAcc := &dct.DCToken{TokenMetaData: metaData}
	dctMetaDataBytes, _ := args.Marshalizer.Marshal(dctDataOnSystemAcc)
	_ = systemAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctMetaDataBytes)

	newMetaData := &dct.MetaData{Name: []byte("newName")}
	transferDCTData := &dct.DCToken{Value: big.NewInt(100), TokenMetaData: newMetaData}
	_, err := e.SaveDCTNFTToken([]byte("address"), userAcc, []byte(key), nonce, transferDCTData, false, false)
	assert.Nil(t, err)

	dctDataGet, _, err := e.GetDCTNFTTokenOnDestination(userAcc, []byte(key), nonce)
	assert.Nil(t, err)
	dctData.TokenMetaData = metaData
	dctData.Value = big.NewInt(100)
	assert.Equal(t, dctData, dctDataGet)
}

func TestDctDataStorage_WasAlreadySentToDestinationShard(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	shardCoordinator := &mock.ShardCoordinatorStub{}
	args.ShardCoordinator = shardCoordinator
	e, _ := NewDCTDataStorage(args)

	tickerID := []byte("ticker")
	dstAddress := []byte("dstAddress")
	val, err := e.WasAlreadySentToDestinationShardAndUpdateState(tickerID, 0, dstAddress)
	assert.True(t, val)
	assert.Nil(t, err)

	val, err = e.WasAlreadySentToDestinationShardAndUpdateState(tickerID, 1, dstAddress)
	assert.True(t, val)
	assert.Nil(t, err)

	shardCoordinator.ComputeIdCalled = func(_ []byte) uint32 {
		return core.MetachainShardId
	}
	val, err = e.WasAlreadySentToDestinationShardAndUpdateState(tickerID, 1, dstAddress)
	assert.True(t, val)
	assert.Nil(t, err)

	shardCoordinator.ComputeIdCalled = func(_ []byte) uint32 {
		return 1
	}
	shardCoordinator.NumberOfShardsCalled = func() uint32 {
		return 5
	}
	val, err = e.WasAlreadySentToDestinationShardAndUpdateState(tickerID, 1, dstAddress)
	assert.False(t, val)
	assert.Nil(t, err)

	systemAcc, _ := e.getSystemAccount()
	metaData := &dct.MetaData{
		Name: []byte("test"),
	}
	dctDataOnSystemAcc := &dct.DCToken{TokenMetaData: metaData}
	dctMetaDataBytes, _ := args.Marshalizer.Marshal(dctDataOnSystemAcc)
	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + string(tickerID)
	tokenKey := append([]byte(key), big.NewInt(1).Bytes()...)
	_ = systemAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctMetaDataBytes)

	val, err = e.WasAlreadySentToDestinationShardAndUpdateState(tickerID, 1, dstAddress)
	assert.False(t, val)
	assert.Nil(t, err)

	val, err = e.WasAlreadySentToDestinationShardAndUpdateState(tickerID, 1, dstAddress)
	assert.True(t, val)
	assert.Nil(t, err)

	shardCoordinator.NumberOfShardsCalled = func() uint32 {
		return 10
	}
	val, err = e.WasAlreadySentToDestinationShardAndUpdateState(tickerID, 1, dstAddress)
	assert.True(t, val)
	assert.Nil(t, err)
}

func TestDctDataStorage_SaveNFTMetaDataToSystemAccount(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	shardCoordinator := &mock.ShardCoordinatorStub{}
	args.ShardCoordinator = shardCoordinator
	e, _ := NewDCTDataStorage(args)

	e.flagSaveToSystemAccount.Unset()
	err := e.SaveNFTMetaDataToSystemAccount(nil)
	assert.Nil(t, err)

	e.flagSaveToSystemAccount.Set()
	err = e.SaveNFTMetaDataToSystemAccount(nil)
	assert.Equal(t, err, ErrNilTransactionHandler)

	scr := &smartContractResult.SmartContractResult{
		SndAddr: []byte("address1"),
		RcvAddr: []byte("address2"),
	}

	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.Nil(t, err)

	shardCoordinator.ComputeIdCalled = func(address []byte) uint32 {
		if bytes.Equal(address, scr.SndAddr) {
			return 0
		}
		if bytes.Equal(address, scr.RcvAddr) {
			return 1
		}
		return 2
	}
	shardCoordinator.NumberOfShardsCalled = func() uint32 {
		return 3
	}
	shardCoordinator.SelfIdCalled = func() uint32 {
		return 1
	}

	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.Nil(t, err)

	scr.Data = []byte("function")
	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.Nil(t, err)

	scr.Data = []byte("function@01@02@03@04")
	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.Nil(t, err)

	scr.Data = []byte(core.BuiltInFunctionDCTNFTTransfer + "@01@02@03@04")
	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.NotNil(t, err)

	scr.Data = []byte(core.BuiltInFunctionDCTNFTTransfer + "@01@02@03@00")
	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.Nil(t, err)

	tickerID := []byte("TCK")
	dctData := &dct.DCToken{
		Value: big.NewInt(10),
		TokenMetaData: &dct.MetaData{
			Name: []byte("test"),
		},
	}
	dctMarshalled, _ := args.Marshalizer.Marshal(dctData)
	scr.Data = []byte(core.BuiltInFunctionDCTNFTTransfer + "@" + hex.EncodeToString(tickerID) + "@01@01@" + hex.EncodeToString(dctMarshalled))
	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.Nil(t, err)

	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + string(tickerID)
	tokenKey := append([]byte(key), big.NewInt(1).Bytes()...)
	dctGetData, _, _ := e.getDCTDigitalTokenDataFromSystemAccount(tokenKey)

	assert.Equal(t, dctData.TokenMetaData, dctGetData.TokenMetaData)
}

func TestDctDataStorage_SaveNFTMetaDataToSystemAccountWithMultiTransfer(t *testing.T) {
	t.Parallel()

	args := createMockArgsForNewDCTDataStorage()
	shardCoordinator := &mock.ShardCoordinatorStub{}
	args.ShardCoordinator = shardCoordinator
	e, _ := NewDCTDataStorage(args)

	scr := &smartContractResult.SmartContractResult{
		SndAddr: []byte("address1"),
		RcvAddr: []byte("address2"),
	}

	shardCoordinator.ComputeIdCalled = func(address []byte) uint32 {
		if bytes.Equal(address, scr.SndAddr) {
			return 0
		}
		if bytes.Equal(address, scr.RcvAddr) {
			return 1
		}
		return 2
	}
	shardCoordinator.NumberOfShardsCalled = func() uint32 {
		return 3
	}
	shardCoordinator.SelfIdCalled = func() uint32 {
		return 1
	}

	tickerID := []byte("TCK")
	dctData := &dct.DCToken{
		Value: big.NewInt(10),
		TokenMetaData: &dct.MetaData{
			Name: []byte("test"),
		},
	}
	dctMarshalled, _ := args.Marshalizer.Marshal(dctData)
	scr.Data = []byte(core.BuiltInFunctionMultiDCTNFTTransfer + "@00@" + hex.EncodeToString(tickerID) + "@01@01@" + hex.EncodeToString(dctMarshalled))
	err := e.SaveNFTMetaDataToSystemAccount(scr)
	assert.True(t, errors.Is(err, ErrInvalidArguments))

	scr.Data = []byte(core.BuiltInFunctionMultiDCTNFTTransfer + "@02@" + hex.EncodeToString(tickerID) + "@01@01@" + hex.EncodeToString(dctMarshalled))
	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.True(t, errors.Is(err, ErrInvalidArguments))

	scr.Data = []byte(core.BuiltInFunctionMultiDCTNFTTransfer + "@02@" + hex.EncodeToString(tickerID) + "@02@10@" +
		hex.EncodeToString(tickerID) + "@01@" + hex.EncodeToString(dctMarshalled))
	err = e.SaveNFTMetaDataToSystemAccount(scr)
	assert.Nil(t, err)

	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + string(tickerID)
	tokenKey := append([]byte(key), big.NewInt(1).Bytes()...)
	dctGetData, _, _ := e.getDCTDigitalTokenDataFromSystemAccount(tokenKey)

	assert.Equal(t, dctData.TokenMetaData, dctGetData.TokenMetaData)

	otherTokenKey := append([]byte(key), big.NewInt(2).Bytes()...)
	dctGetData, _, err = e.getDCTDigitalTokenDataFromSystemAccount(otherTokenKey)
	assert.Nil(t, dctGetData)
	assert.Nil(t, err)
}
