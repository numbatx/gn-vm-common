package builtInFunctions

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-core/data/vm"
	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createNftCreateWithStubArguments() *dctNFTCreate {
	nftCreate, _ := NewDCTNFTCreateFunc(
		1,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		&mock.DCTRoleHandlerStub{},
		createNewDCTDataStorageHandler(),
		&mock.AccountsStub{},
		0,
		&mock.EpochNotifierStub{},
	)

	return nftCreate
}

func TestNewDCTNFTCreateFunc_NilArgumentsShouldErr(t *testing.T) {
	t.Parallel()

	nftCreate, err := NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		nil,
		&mock.GlobalSettingsHandlerStub{},
		&mock.DCTRoleHandlerStub{},
		createNewDCTDataStorageHandler(),
		&mock.AccountsStub{},
		0,
		&mock.EpochNotifierStub{},
	)
	assert.True(t, check.IfNil(nftCreate))
	assert.Equal(t, ErrNilMarshalizer, err)

	nftCreate, err = NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		nil,
		&mock.DCTRoleHandlerStub{},
		createNewDCTDataStorageHandler(),
		&mock.AccountsStub{},
		0,
		&mock.EpochNotifierStub{},
	)
	assert.True(t, check.IfNil(nftCreate))
	assert.Equal(t, ErrNilGlobalSettingsHandler, err)

	nftCreate, err = NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		nil,
		createNewDCTDataStorageHandler(),
		&mock.AccountsStub{},
		0,
		&mock.EpochNotifierStub{},
	)
	assert.True(t, check.IfNil(nftCreate))
	assert.Equal(t, ErrNilRolesHandler, err)

	nftCreate, err = NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		&mock.DCTRoleHandlerStub{},
		nil,
		&mock.AccountsStub{},
		0,
		&mock.EpochNotifierStub{},
	)
	assert.True(t, check.IfNil(nftCreate))
	assert.Equal(t, ErrNilDCTNFTStorageHandler, err)

	nftCreate, err = NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		&mock.DCTRoleHandlerStub{},
		createNewDCTDataStorageHandler(),
		&mock.AccountsStub{},
		0,
		nil,
	)
	assert.True(t, check.IfNil(nftCreate))
	assert.Equal(t, ErrNilEpochHandler, err)
}

func TestNewDCTNFTCreateFunc(t *testing.T) {
	t.Parallel()

	nftCreate, err := NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		&mock.DCTRoleHandlerStub{},
		createNewDCTDataStorageHandler(),
		&mock.AccountsStub{},
		0,
		&mock.EpochNotifierStub{},
	)
	assert.False(t, check.IfNil(nftCreate))
	assert.Nil(t, err)
}

func TestDctNFTCreate_SetNewGasConfig(t *testing.T) {
	t.Parallel()

	nftCreate := createNftCreateWithStubArguments()
	nftCreate.SetNewGasConfig(nil)
	assert.Equal(t, uint64(1), nftCreate.funcGasCost)
	assert.Equal(t, vmcommon.BaseOperationCost{}, nftCreate.gasConfig)

	gasCost := createMockGasCost()
	nftCreate.SetNewGasConfig(&gasCost)
	assert.Equal(t, gasCost.BuiltInCost.DCTNFTCreate, nftCreate.funcGasCost)
	assert.Equal(t, gasCost.BaseOperationCost, nftCreate.gasConfig)
}

func TestDctNFTCreate_ProcessBuiltinFunctionInvalidArguments(t *testing.T) {
	t.Parallel()

	nftCreate := createNftCreateWithStubArguments()
	sender := mock.NewAccountWrapMock([]byte("address"))
	vmOutput, err := nftCreate.ProcessBuiltinFunction(sender, nil, nil)
	assert.Nil(t, vmOutput)
	assert.Equal(t, ErrNilVmInput, err)

	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: []byte("caller"),
			CallValue:  big.NewInt(0),
			Arguments:  [][]byte{[]byte("arg1"), []byte("arg2")},
		},
		RecipientAddr: []byte("recipient"),
	}
	vmOutput, err = nftCreate.ProcessBuiltinFunction(sender, nil, vmInput)
	assert.Nil(t, vmOutput)
	assert.Equal(t, ErrInvalidRcvAddr, err)

	vmInput = &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: sender.AddressBytes(),
			CallValue:  big.NewInt(0),
			Arguments:  [][]byte{[]byte("arg1"), []byte("arg2")},
		},
		RecipientAddr: sender.AddressBytes(),
	}
	vmOutput, err = nftCreate.ProcessBuiltinFunction(nil, nil, vmInput)
	assert.Nil(t, vmOutput)
	assert.Equal(t, ErrNilUserAccount, err)

	vmInput = &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: sender.AddressBytes(),
			CallValue:  big.NewInt(0),
			Arguments:  [][]byte{[]byte("arg1"), []byte("arg2")},
		},
		RecipientAddr: sender.AddressBytes(),
	}
	vmOutput, err = nftCreate.ProcessBuiltinFunction(sender, nil, vmInput)
	assert.Nil(t, vmOutput)
	assert.Equal(t, ErrNotEnoughGas, err)

	vmInput = &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:  sender.AddressBytes(),
			CallValue:   big.NewInt(0),
			Arguments:   [][]byte{[]byte("arg1"), []byte("arg2")},
			GasProvided: 1,
		},
		RecipientAddr: sender.AddressBytes(),
	}
	vmOutput, err = nftCreate.ProcessBuiltinFunction(sender, nil, vmInput)
	assert.Nil(t, vmOutput)
	assert.True(t, errors.Is(err, ErrInvalidArguments))
}

func TestDctNFTCreate_ProcessBuiltinFunctionNotAllowedToExecute(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected error")
	dctDataStorage := createNewDCTDataStorageHandler()
	nftCreate, _ := NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		&mock.DCTRoleHandlerStub{
			CheckAllowedToExecuteCalled: func(account vmcommon.UserAccountHandler, tokenID []byte, action []byte) error {
				return expectedErr
			},
		},
		dctDataStorage,
		dctDataStorage.accounts,
		0,
		&mock.EpochNotifierStub{},
	)
	sender := mock.NewAccountWrapMock([]byte("address"))
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: sender.AddressBytes(),
			CallValue:  big.NewInt(0),
			Arguments:  make([][]byte, 7),
		},
		RecipientAddr: sender.AddressBytes(),
	}
	vmOutput, err := nftCreate.ProcessBuiltinFunction(sender, nil, vmInput)
	assert.Nil(t, vmOutput)
	assert.Equal(t, expectedErr, err)
}

func TestDctNFTCreate_ProcessBuiltinFunctionShouldWork(t *testing.T) {
	t.Parallel()

	dctDataStorage := createNewDCTDataStorageHandler()
	nftCreate, _ := NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		&mock.DCTRoleHandlerStub{},
		dctDataStorage,
		dctDataStorage.accounts,
		0,
		&mock.EpochNotifierStub{},
	)
	address := bytes.Repeat([]byte{1}, 32)
	sender := mock.NewUserAccount(address)
	//add some data in the trie, otherwise the creation will fail (it won't happen in real case usage as the create NFT
	//will be called after the creation permission was set in the account's data)
	_ = sender.AccountDataHandler().SaveKeyValue([]byte("key"), []byte("value"))

	token := "token"
	quantity := big.NewInt(2)
	name := "name"
	royalties := 100 //1%
	hash := []byte("12345678901234567890123456789012")
	attributes := []byte("attributes")
	uris := [][]byte{[]byte("uri1"), []byte("uri2")}
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: sender.AddressBytes(),
			CallValue:  big.NewInt(0),
			Arguments: [][]byte{
				[]byte(token),
				quantity.Bytes(),
				[]byte(name),
				big.NewInt(int64(royalties)).Bytes(),
				hash,
				attributes,
				uris[0],
				uris[1],
			},
		},
		RecipientAddr: sender.AddressBytes(),
	}
	vmOutput, err := nftCreate.ProcessBuiltinFunction(sender, nil, vmInput)
	assert.Nil(t, err)
	require.NotNil(t, vmOutput)

	createdDct, latestNonce := readNFTData(t, sender, nftCreate.marshalizer, []byte(token), 1, address)
	assert.Equal(t, uint64(1), latestNonce)
	expectedDct := &dct.DCToken{
		Type:  uint32(core.NonFungible),
		Value: quantity,
	}
	assert.Equal(t, expectedDct, createdDct)

	tokenMetaData := &dct.MetaData{
		Nonce:      1,
		Name:       []byte(name),
		Creator:    address,
		Royalties:  uint32(royalties),
		Hash:       hash,
		URIs:       uris,
		Attributes: attributes,
	}

	tokenKey := []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + token)
	tokenKey = append(tokenKey, big.NewInt(1).Bytes()...)

	metaData, _ := dctDataStorage.getDCTMetaDataFromSystemAccount(tokenKey)
	assert.Equal(t, tokenMetaData, metaData)
}

func TestDctNFTCreate_ProcessBuiltinFunctionWithExecByCaller(t *testing.T) {
	t.Parallel()

	accounts := createAccountsAdapterWithMap()
	dctDataStorage := createNewDCTDataStorageHandlerWithArgs(&mock.GlobalSettingsHandlerStub{}, accounts)
	_ = dctDataStorage.flagSaveToSystemAccount.SetReturningPrevious()
	nftCreate, _ := NewDCTNFTCreateFunc(
		0,
		vmcommon.BaseOperationCost{},
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		&mock.DCTRoleHandlerStub{},
		dctDataStorage,
		dctDataStorage.accounts,
		0,
		&mock.EpochNotifierStub{},
	)
	address := bytes.Repeat([]byte{1}, 32)
	userAddress := bytes.Repeat([]byte{2}, 32)
	token := "token"
	quantity := big.NewInt(2)
	name := "name"
	royalties := 100 //1%
	hash := []byte("12345678901234567890123456789012")
	attributes := []byte("attributes")
	uris := [][]byte{[]byte("uri1"), []byte("uri2")}
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: userAddress,
			CallValue:  big.NewInt(0),
			Arguments: [][]byte{
				[]byte(token),
				quantity.Bytes(),
				[]byte(name),
				big.NewInt(int64(royalties)).Bytes(),
				hash,
				attributes,
				uris[0],
				uris[1],
				address,
			},
			CallType: vm.ExecOnDestByCaller,
		},
		RecipientAddr: userAddress,
	}
	vmOutput, err := nftCreate.ProcessBuiltinFunction(nil, nil, vmInput)
	assert.Nil(t, err)
	require.NotNil(t, vmOutput)

	roleAcc, _ := nftCreate.getAccount(address)

	createdDct, latestNonce := readNFTData(t, roleAcc, nftCreate.marshalizer, []byte(token), 1, address)
	assert.Equal(t, uint64(1), latestNonce)
	expectedDct := &dct.DCToken{
		Type:  uint32(core.NonFungible),
		Value: quantity,
	}
	assert.Equal(t, expectedDct, createdDct)

	tokenMetaData := &dct.MetaData{
		Nonce:      1,
		Name:       []byte(name),
		Creator:    userAddress,
		Royalties:  uint32(royalties),
		Hash:       hash,
		URIs:       uris,
		Attributes: attributes,
	}

	tokenKey := []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + token)
	tokenKey = append(tokenKey, big.NewInt(1).Bytes()...)

	metaData, _ := dctDataStorage.getDCTMetaDataFromSystemAccount(tokenKey)
	assert.Equal(t, tokenMetaData, metaData)
}

func readNFTData(t *testing.T, account vmcommon.UserAccountHandler, marshalizer vmcommon.Marshalizer, tokenID []byte, nonce uint64, _ []byte) (*dct.DCToken, uint64) {
	nonceKey := getNonceKey(tokenID)
	latestNonceBytes, err := account.(vmcommon.UserAccountHandler).AccountDataHandler().RetrieveValue(nonceKey)
	require.Nil(t, err)
	latestNonce := big.NewInt(0).SetBytes(latestNonceBytes).Uint64()

	createdTokenID := []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier)
	createdTokenID = append(createdTokenID, tokenID...)
	tokenKey := computeDCTNFTTokenKey(createdTokenID, nonce)
	data, err := account.(vmcommon.UserAccountHandler).AccountDataHandler().RetrieveValue(tokenKey)
	require.Nil(t, err)

	dctData := &dct.DCToken{}
	err = marshalizer.Unmarshal(dctData, data)
	require.Nil(t, err)

	return dctData, latestNonce
}
