package builtInFunctions

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var keyPrefix = []byte(baseDCTKeyPrefix)

func createNftTransferWithStubArguments() *dctNFTTransfer {
	nftTransfer, _ := NewDCTNFTTransferFunc(
		0,
		&mock.MarshalizerMock{},
		&mock.GlobalSettingsHandlerStub{},
		&mock.AccountsStub{},
		&mock.ShardCoordinatorStub{},
		vmcommon.BaseOperationCost{},
		&mock.DCTRoleHandlerStub{},
		createNewDCTDataStorageHandler(),
		&mock.EnableEpochsHandlerStub{
			IsTransferToMetaFlagEnabledField:                     false,
			IsSaveToSystemAccountFlagEnabledField:                true,
			IsCheckCorrectTokenIDForTransferRoleFlagEnabledField: true,
		},
	)

	return nftTransfer
}

func createNFTTransferAndStorageHandler(selfShard, numShards uint32, globalSettingsHandler vmcommon.ExtendedDCTGlobalSettingsHandler, enableEpochsHandler vmcommon.EnableEpochsHandler) (*dctNFTTransfer, *dctDataStorage) {
	marshaller := &mock.MarshalizerMock{}
	shardCoordinator := mock.NewMultiShardsCoordinatorMock(numShards)
	shardCoordinator.CurrentShard = selfShard
	shardCoordinator.ComputeIdCalled = func(address []byte) uint32 {
		lastByte := uint32(address[len(address)-1])
		return lastByte
	}
	mapAccounts := make(map[string]vmcommon.UserAccountHandler)
	accounts := &mock.AccountsStub{
		LoadAccountCalled: func(address []byte) (vmcommon.AccountHandler, error) {
			_, ok := mapAccounts[string(address)]
			if !ok {
				mapAccounts[string(address)] = mock.NewUserAccount(address)
			}
			return mapAccounts[string(address)], nil
		},
		GetExistingAccountCalled: func(address []byte) (vmcommon.AccountHandler, error) {
			_, ok := mapAccounts[string(address)]
			if !ok {
				mapAccounts[string(address)] = mock.NewUserAccount(address)
			}
			return mapAccounts[string(address)], nil
		},
	}

	dctStorageHandler := createNewDCTDataStorageHandlerWithArgs(globalSettingsHandler, accounts, enableEpochsHandler)
	nftTransfer, _ := NewDCTNFTTransferFunc(
		1,
		marshaller,
		globalSettingsHandler,
		accounts,
		shardCoordinator,
		vmcommon.BaseOperationCost{},
		&mock.DCTRoleHandlerStub{
			CheckAllowedToExecuteCalled: func(account vmcommon.UserAccountHandler, tokenID []byte, action []byte) error {
				if bytes.Equal(action, []byte(core.DCTRoleTransfer)) {
					return ErrActionNotAllowed
				}
				return nil
			},
		},
		dctStorageHandler,
		enableEpochsHandler,
	)

	return nftTransfer, dctStorageHandler
}

func createNftTransferWithMockArguments(selfShard uint32, numShards uint32, globalSettingsHandler vmcommon.ExtendedDCTGlobalSettingsHandler) *dctNFTTransfer {
	nftTransfer, _ := createNFTTransferAndStorageHandler(selfShard, numShards, globalSettingsHandler, &mock.EnableEpochsHandlerStub{
		IsTransferToMetaFlagEnabledField:        true,
		IsCheckTransferFlagEnabledField:         true,
		IsCheckFrozenCollectionFlagEnabledField: true,
	})
	return nftTransfer
}

func createMockGasCost() vmcommon.GasCost {
	return vmcommon.GasCost{
		BaseOperationCost: vmcommon.BaseOperationCost{
			StorePerByte:      10,
			ReleasePerByte:    20,
			DataCopyPerByte:   30,
			PersistPerByte:    40,
			CompilePerByte:    50,
			AoTPreparePerByte: 60,
		},
		BuiltInCost: vmcommon.BuiltInCost{
			ChangeOwnerAddress:       70,
			ClaimDeveloperRewards:    80,
			SaveUserName:             90,
			SaveKeyValue:             100,
			DCTTransfer:             110,
			DCTBurn:                 120,
			DCTLocalMint:            130,
			DCTLocalBurn:            140,
			DCTNFTCreate:            150,
			DCTNFTAddQuantity:       160,
			DCTNFTBurn:              170,
			DCTNFTTransfer:          180,
			DCTNFTChangeCreateOwner: 190,
			DCTNFTUpdateAttributes:  200,
			DCTNFTAddURI:            210,
			DCTNFTMultiTransfer:     220,
		},
	}
}

func createDCTNFTToken(
	tokenName []byte,
	nftType core.DCTType,
	nonce uint64,
	value *big.Int,
	marshaller vmcommon.Marshalizer,
	account vmcommon.UserAccountHandler,
) {
	tokenId := append(keyPrefix, tokenName...)
	dctNFTTokenKey := computeDCTNFTTokenKey(tokenId, nonce)
	dctData := &dct.DCToken{
		Type:  uint32(nftType),
		Value: value,
	}

	if nonce > 0 {
		dctData.TokenMetaData = &dct.MetaData{
			URIs:  [][]byte{[]byte("uri")},
			Nonce: nonce,
			Hash:  []byte("NFT hash"),
		}
	}

	buff, _ := marshaller.Marshal(dctData)

	_ = account.AccountDataHandler().SaveKeyValue(dctNFTTokenKey, buff)
}

func testNFTTokenShouldExist(
	tb testing.TB,
	marshaller vmcommon.Marshalizer,
	account vmcommon.AccountHandler,
	tokenName []byte,
	nonce uint64,
	expectedValue *big.Int,
) {
	tokenId := append(keyPrefix, tokenName...)
	dctNFTTokenKey := computeDCTNFTTokenKey(tokenId, nonce)
	dctData := &dct.DCToken{Value: big.NewInt(0), Type: uint32(core.Fungible)}
	marshaledData, _, _ := account.(vmcommon.UserAccountHandler).AccountDataHandler().RetrieveValue(dctNFTTokenKey)
	_ = marshaller.Unmarshal(dctData, marshaledData)
	assert.Equal(tb, expectedValue, dctData.Value)
}

func TestNewDCTNFTTransferFunc(t *testing.T) {
	t.Parallel()

	t.Run("nil marshaller should error", func(t *testing.T) {
		t.Parallel()

		nftTransfer, err := NewDCTNFTTransferFunc(
			0,
			nil,
			&mock.GlobalSettingsHandlerStub{},
			&mock.AccountsStub{},
			&mock.ShardCoordinatorStub{},
			vmcommon.BaseOperationCost{},
			&mock.DCTRoleHandlerStub{},
			createNewDCTDataStorageHandler(),
			&mock.EnableEpochsHandlerStub{},
		)
		assert.True(t, check.IfNil(nftTransfer))
		assert.Equal(t, ErrNilMarshalizer, err)
	})
	t.Run("nil global settings handler should error", func(t *testing.T) {
		t.Parallel()

		nftTransfer, err := NewDCTNFTTransferFunc(
			0,
			&mock.MarshalizerMock{},
			nil,
			&mock.AccountsStub{},
			&mock.ShardCoordinatorStub{},
			vmcommon.BaseOperationCost{},
			&mock.DCTRoleHandlerStub{},
			createNewDCTDataStorageHandler(),
			&mock.EnableEpochsHandlerStub{},
		)
		assert.True(t, check.IfNil(nftTransfer))
		assert.Equal(t, ErrNilGlobalSettingsHandler, err)
	})
	t.Run("nil accounts adapter should error", func(t *testing.T) {
		t.Parallel()

		nftTransfer, err := NewDCTNFTTransferFunc(
			0,
			&mock.MarshalizerMock{},
			&mock.GlobalSettingsHandlerStub{},
			nil,
			&mock.ShardCoordinatorStub{},
			vmcommon.BaseOperationCost{},
			&mock.DCTRoleHandlerStub{},
			createNewDCTDataStorageHandler(),
			&mock.EnableEpochsHandlerStub{},
		)
		assert.True(t, check.IfNil(nftTransfer))
		assert.Equal(t, ErrNilAccountsAdapter, err)
	})
	t.Run("nil shard coordinator should error", func(t *testing.T) {
		t.Parallel()

		nftTransfer, err := NewDCTNFTTransferFunc(
			0,
			&mock.MarshalizerMock{},
			&mock.GlobalSettingsHandlerStub{},
			&mock.AccountsStub{},
			nil,
			vmcommon.BaseOperationCost{},
			&mock.DCTRoleHandlerStub{},
			createNewDCTDataStorageHandler(),
			&mock.EnableEpochsHandlerStub{},
		)
		assert.True(t, check.IfNil(nftTransfer))
		assert.Equal(t, ErrNilShardCoordinator, err)
	})
	t.Run("nil roles handler should error", func(t *testing.T) {
		t.Parallel()

		nftTransfer, err := NewDCTNFTTransferFunc(
			0,
			&mock.MarshalizerMock{},
			&mock.GlobalSettingsHandlerStub{},
			&mock.AccountsStub{},
			&mock.ShardCoordinatorStub{},
			vmcommon.BaseOperationCost{},
			nil,
			createNewDCTDataStorageHandler(),
			&mock.EnableEpochsHandlerStub{},
		)
		assert.True(t, check.IfNil(nftTransfer))
		assert.Equal(t, ErrNilRolesHandler, err)
	})
	t.Run("nil dct storage handler should error", func(t *testing.T) {
		t.Parallel()

		nftTransfer, err := NewDCTNFTTransferFunc(
			0,
			&mock.MarshalizerMock{},
			&mock.GlobalSettingsHandlerStub{},
			&mock.AccountsStub{},
			&mock.ShardCoordinatorStub{},
			vmcommon.BaseOperationCost{},
			&mock.DCTRoleHandlerStub{},
			nil,
			&mock.EnableEpochsHandlerStub{},
		)
		assert.True(t, check.IfNil(nftTransfer))
		assert.Equal(t, ErrNilDCTNFTStorageHandler, err)
	})
	t.Run("nil enable epochs handler should error", func(t *testing.T) {
		t.Parallel()

		nftTransfer, err := NewDCTNFTTransferFunc(
			0,
			&mock.MarshalizerMock{},
			&mock.GlobalSettingsHandlerStub{},
			&mock.AccountsStub{},
			&mock.ShardCoordinatorStub{},
			vmcommon.BaseOperationCost{},
			&mock.DCTRoleHandlerStub{},
			createNewDCTDataStorageHandler(),
			nil,
		)
		assert.True(t, check.IfNil(nftTransfer))
		assert.Equal(t, ErrNilEnableEpochsHandler, err)
	})
	t.Run("should work", func(t *testing.T) {
		t.Parallel()

		nftTransfer, err := NewDCTNFTTransferFunc(
			0,
			&mock.MarshalizerMock{},
			&mock.GlobalSettingsHandlerStub{},
			&mock.AccountsStub{},
			&mock.ShardCoordinatorStub{},
			vmcommon.BaseOperationCost{},
			&mock.DCTRoleHandlerStub{},
			createNewDCTDataStorageHandler(),
			&mock.EnableEpochsHandlerStub{},
		)
		assert.False(t, check.IfNil(nftTransfer))
		assert.Nil(t, err)
	})
}

func TestDctNFTTransfer_SetPayable(t *testing.T) {
	t.Parallel()

	nftTransfer := createNftTransferWithStubArguments()
	err := nftTransfer.SetPayableChecker(nil)
	assert.Equal(t, ErrNilPayableHandler, err)

	handler := &mock.PayableHandlerStub{}
	err = nftTransfer.SetPayableChecker(handler)
	assert.Nil(t, err)
	assert.True(t, handler == nftTransfer.payableHandler) // pointer testing
}

func TestDctNFTTransfer_SetNewGasConfig(t *testing.T) {
	t.Parallel()

	nftTransfer := createNftTransferWithStubArguments()
	nftTransfer.SetNewGasConfig(nil)
	assert.Equal(t, uint64(0), nftTransfer.funcGasCost)
	assert.Equal(t, vmcommon.BaseOperationCost{}, nftTransfer.gasConfig)

	gasCost := createMockGasCost()
	nftTransfer.SetNewGasConfig(&gasCost)
	assert.Equal(t, gasCost.BuiltInCost.DCTNFTTransfer, nftTransfer.funcGasCost)
	assert.Equal(t, gasCost.BaseOperationCost, nftTransfer.gasConfig)
}

func TestDctNFTTransfer_ProcessBuiltinFunctionInvalidArgumentsShouldErr(t *testing.T) {
	t.Parallel()

	nftTransfer := createNftTransferWithStubArguments()
	vmOutput, err := nftTransfer.ProcessBuiltinFunction(&mock.UserAccountStub{}, &mock.UserAccountStub{}, nil)
	assert.Nil(t, vmOutput)
	assert.Equal(t, ErrNilVmInput, err)

	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue: big.NewInt(0),
			Arguments: [][]byte{[]byte("arg1"), []byte("arg2")},
		},
	}
	vmOutput, err = nftTransfer.ProcessBuiltinFunction(&mock.UserAccountStub{}, &mock.UserAccountStub{}, vmInput)
	assert.Nil(t, vmOutput)
	assert.Equal(t, ErrInvalidArguments, err)

	nftTransfer.shardCoordinator = &mock.ShardCoordinatorStub{ComputeIdCalled: func(address []byte) uint32 {
		return core.MetachainShardId
	}}

	tokenName := []byte("token")
	senderAddress := bytes.Repeat([]byte{2}, 32)
	vmInput = &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, big.NewInt(1).Bytes(), big.NewInt(1).Bytes(), core.DCTSCAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}
	vmOutput, err = nftTransfer.ProcessBuiltinFunction(&mock.UserAccountStub{}, &mock.UserAccountStub{}, vmInput)
	assert.Nil(t, vmOutput)
	assert.Equal(t, ErrInvalidRcvAddr, err)
}

func TestDctNFTTransfer_SenderDoesNotHaveNFT(t *testing.T) {
	t.Parallel()

	nftTransfer := createNftTransferWithMockArguments(0, 1, &mock.GlobalSettingsHandlerStub{})
	_ = nftTransfer.SetPayableChecker(
		&mock.PayableHandlerStub{
			IsPayableCalled: func(address []byte) (bool, error) {
				return true, nil
			},
		})
	senderAddress := bytes.Repeat([]byte{2}, 32)
	destinationAddress := bytes.Repeat([]byte{1}, 32)
	sender, err := nftTransfer.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)
	destination, err := nftTransfer.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(0)

	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}

	_, err = nftTransfer.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	require.Equal(t, err, ErrNewNFTDataOnSenderAddress)
}

func TestDctNFTTransfer_ProcessWithZeroValue(t *testing.T) {
	t.Parallel()

	nftTransfer := createNftTransferWithMockArguments(0, 1, &mock.GlobalSettingsHandlerStub{})

	senderAddress := bytes.Repeat([]byte{2}, 32)
	destinationAddress := bytes.Repeat([]byte{1}, 32)

	sender, err := nftTransfer.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)
	destination, err := nftTransfer.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, nftTransfer.marshaller, sender.(vmcommon.UserAccountHandler))
	_ = nftTransfer.accounts.SaveAccount(sender)
	_ = nftTransfer.accounts.SaveAccount(destination)
	_, _ = nftTransfer.accounts.Commit()

	// reload accounts
	sender, err = nftTransfer.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)
	destination, err = nftTransfer.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	scCallFunctionAsHex := hex.EncodeToString([]byte("functionToCall"))
	scCallArg := hex.EncodeToString([]byte("arg"))
	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(0).Bytes()
	scCallArgs := [][]byte{[]byte(scCallFunctionAsHex), []byte(scCallArg)}
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}
	vmInput.Arguments = append(vmInput.Arguments, scCallArgs...)

	_, err = nftTransfer.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	require.Equal(t, err, ErrInvalidNFTQuantity)
}

func TestDctNFTTransfer_ProcessBuiltinFunctionOnSameShardWithScCall(t *testing.T) {
	t.Parallel()

	nftTransfer := createNftTransferWithMockArguments(0, 1, &mock.GlobalSettingsHandlerStub{})

	payableChecker, _ := NewPayableCheckFunc(
		&mock.PayableHandlerStub{
			IsPayableCalled: func(address []byte) (bool, error) {
				return true, nil
			},
		}, &mock.EnableEpochsHandlerStub{
			IsFixAsyncCallbackCheckFlagEnabledField: true,
			IsCheckFunctionArgumentFlagEnabledField: true,
		})

	_ = nftTransfer.SetPayableChecker(payableChecker)
	senderAddress := bytes.Repeat([]byte{2}, 32)
	destinationAddress := bytes.Repeat([]byte{0}, 32)
	destinationAddress[25] = 1
	sender, err := nftTransfer.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)
	destination, err := nftTransfer.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, nftTransfer.marshaller, sender.(vmcommon.UserAccountHandler))
	_ = nftTransfer.accounts.SaveAccount(sender)
	_ = nftTransfer.accounts.SaveAccount(destination)
	_, _ = nftTransfer.accounts.Commit()

	// reload accounts
	sender, err = nftTransfer.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)
	destination, err = nftTransfer.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	scCallFunctionAsHex := hex.EncodeToString([]byte("functionToCall"))
	scCallArg := hex.EncodeToString([]byte("arg"))
	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	scCallArgs := [][]byte{[]byte(scCallFunctionAsHex), []byte(scCallArg)}
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}
	vmInput.Arguments = append(vmInput.Arguments, scCallArgs...)

	vmOutput, err := nftTransfer.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	require.Nil(t, err)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)

	_ = nftTransfer.accounts.SaveAccount(sender)
	_, _ = nftTransfer.accounts.Commit()

	// reload accounts
	sender, err = nftTransfer.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)
	destination, err = nftTransfer.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	testNFTTokenShouldExist(t, nftTransfer.marshaller, sender, tokenName, tokenNonce, big.NewInt(2)) // 3 initial - 1 transferred
	testNFTTokenShouldExist(t, nftTransfer.marshaller, destination, tokenName, tokenNonce, big.NewInt(1))
	funcName, args := extractScResultsFromVmOutput(t, vmOutput)
	assert.Equal(t, scCallFunctionAsHex, funcName)
	require.Equal(t, 1, len(args))
	require.Equal(t, []byte(scCallArg), args[0])
}

func TestDctNFTTransfer_ProcessBuiltinFunctionOnCrossShardsDestinationDoesNotHoldingNFTWithSCCall(t *testing.T) {
	t.Parallel()

	payableHandler := &mock.PayableHandlerStub{
		IsPayableCalled: func(address []byte) (bool, error) {
			return true, nil
		},
	}

	nftTransferSenderShard := createNftTransferWithMockArguments(1, 2, &mock.GlobalSettingsHandlerStub{})
	_ = nftTransferSenderShard.SetPayableChecker(payableHandler)

	nftTransferDestinationShard := createNftTransferWithMockArguments(0, 2, &mock.GlobalSettingsHandlerStub{})
	_ = nftTransferDestinationShard.SetPayableChecker(payableHandler)

	senderAddress := bytes.Repeat([]byte{1}, 32)
	destinationAddress := bytes.Repeat([]byte{0}, 32)
	destinationAddress[25] = 1
	sender, err := nftTransferSenderShard.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, nftTransferSenderShard.marshaller, sender.(vmcommon.UserAccountHandler))
	_ = nftTransferSenderShard.accounts.SaveAccount(sender)
	_, _ = nftTransferSenderShard.accounts.Commit()

	// reload sender account
	sender, err = nftTransferSenderShard.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	scCallFunctionAsHex := hex.EncodeToString([]byte("functionToCall"))
	scCallArg := hex.EncodeToString([]byte("arg"))
	scCallArgs := [][]byte{[]byte(scCallFunctionAsHex), []byte(scCallArg)}
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}
	vmInput.Arguments = append(vmInput.Arguments, scCallArgs...)

	vmOutput, err := nftTransferSenderShard.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), nil, vmInput)
	require.Nil(t, err)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)

	_ = nftTransferSenderShard.accounts.SaveAccount(sender)
	_, _ = nftTransferSenderShard.accounts.Commit()

	// reload sender account
	sender, err = nftTransferSenderShard.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	testNFTTokenShouldExist(t, nftTransferSenderShard.marshaller, sender, tokenName, tokenNonce, big.NewInt(2)) // 3 initial - 1 transferred

	funcName, args := extractScResultsFromVmOutput(t, vmOutput)

	destination, err := nftTransferDestinationShard.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	vmInput = &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: senderAddress,
			Arguments:  args,
		},
		RecipientAddr: destinationAddress,
	}

	vmOutput, err = nftTransferDestinationShard.ProcessBuiltinFunction(nil, destination.(vmcommon.UserAccountHandler), vmInput)
	require.Nil(t, err)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = nftTransferDestinationShard.accounts.SaveAccount(destination)
	_, _ = nftTransferDestinationShard.accounts.Commit()

	destination, err = nftTransferDestinationShard.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	testNFTTokenShouldExist(t, nftTransferDestinationShard.marshaller, destination, tokenName, tokenNonce, big.NewInt(1))
	funcName, args = extractScResultsFromVmOutput(t, vmOutput)
	assert.Equal(t, scCallFunctionAsHex, funcName)
	require.Equal(t, 1, len(args))
	require.Equal(t, []byte(scCallArg), args[0])
}

func TestDctNFTTransfer_ProcessBuiltinFunctionOnCrossShardsDestinationHoldsNFT(t *testing.T) {
	t.Parallel()

	payableHandler := &mock.PayableHandlerStub{
		IsPayableCalled: func(address []byte) (bool, error) {
			return true, nil
		},
	}

	nftTransferSenderShard := createNftTransferWithMockArguments(0, 2, &mock.GlobalSettingsHandlerStub{})
	_ = nftTransferSenderShard.SetPayableChecker(payableHandler)

	nftTransferDestinationShard := createNftTransferWithMockArguments(1, 2, &mock.GlobalSettingsHandlerStub{})
	_ = nftTransferDestinationShard.SetPayableChecker(payableHandler)

	senderAddress := bytes.Repeat([]byte{2}, 32) // sender is in the same shard
	destinationAddress := bytes.Repeat([]byte{1}, 32)
	sender, err := nftTransferSenderShard.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, nftTransferSenderShard.marshaller, sender.(vmcommon.UserAccountHandler))
	_ = nftTransferSenderShard.accounts.SaveAccount(sender)
	_, _ = nftTransferSenderShard.accounts.Commit()

	// reload sender account
	sender, err = nftTransferSenderShard.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}

	vmOutput, err := nftTransferSenderShard.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), nil, vmInput)
	require.Nil(t, err)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)

	_ = nftTransferSenderShard.accounts.SaveAccount(sender)
	_, _ = nftTransferSenderShard.accounts.Commit()

	// reload sender account
	sender, err = nftTransferSenderShard.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	testNFTTokenShouldExist(t, nftTransferSenderShard.marshaller, sender, tokenName, tokenNonce, big.NewInt(2)) // 3 initial - 1 transferred

	_, args := extractScResultsFromVmOutput(t, vmOutput)

	destinationNumTokens := big.NewInt(1000)
	destination, err := nftTransferDestinationShard.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, destinationNumTokens, nftTransferDestinationShard.marshaller, destination.(vmcommon.UserAccountHandler))
	_ = nftTransferDestinationShard.accounts.SaveAccount(destination)
	_, _ = nftTransferDestinationShard.accounts.Commit()

	destination, err = nftTransferDestinationShard.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	vmInput = &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: senderAddress,
			Arguments:  args,
		},
		RecipientAddr: destinationAddress,
	}

	vmOutput, err = nftTransferDestinationShard.ProcessBuiltinFunction(nil, destination.(vmcommon.UserAccountHandler), vmInput)
	require.Nil(t, err)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = nftTransferDestinationShard.accounts.SaveAccount(destination)
	_, _ = nftTransferDestinationShard.accounts.Commit()

	destination, err = nftTransferDestinationShard.accounts.LoadAccount(destinationAddress)
	require.Nil(t, err)

	expected := big.NewInt(0).Add(destinationNumTokens, big.NewInt(1))
	testNFTTokenShouldExist(t, nftTransferDestinationShard.marshaller, destination, tokenName, tokenNonce, expected)
}

func TestDCTNFTTransfer_SndDstFrozen(t *testing.T) {
	t.Parallel()

	globalSettings := &mock.GlobalSettingsHandlerStub{}
	transferFunc := createNftTransferWithMockArguments(0, 1, globalSettings)
	_ = transferFunc.SetPayableChecker(&mock.PayableHandlerStub{})

	senderAddress := bytes.Repeat([]byte{2}, 32) // sender is in the same shard
	destinationAddress := bytes.Repeat([]byte{1}, 32)
	destinationAddress[31] = 0
	sender, err := transferFunc.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, transferFunc.marshaller, sender.(vmcommon.UserAccountHandler))
	dctFrozen := DCTUserMetadata{Frozen: true}

	_ = transferFunc.accounts.SaveAccount(sender)
	_, _ = transferFunc.accounts.Commit()
	// reload sender account
	sender, err = transferFunc.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}

	destination, _ := transferFunc.accounts.LoadAccount(destinationAddress)
	tokenId := append(keyPrefix, tokenName...)
	dctKey := computeDCTNFTTokenKey(tokenId, tokenNonce)
	dctToken := &dct.DCToken{Value: big.NewInt(0), Properties: dctFrozen.ToBytes()}
	marshaledData, _ := transferFunc.marshaller.Marshal(dctToken)
	_ = destination.(vmcommon.UserAccountHandler).AccountDataHandler().SaveKeyValue(dctKey, marshaledData)
	_ = transferFunc.accounts.SaveAccount(destination)
	_, _ = transferFunc.accounts.Commit()

	_, err = transferFunc.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	assert.Equal(t, ErrDCTIsFrozenForAccount, err)

	vmInput.ReturnCallAfterError = true
	_, err = transferFunc.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	assert.Nil(t, err)
}

func TestDCTNFTTransfer_WithLimitedTransfer(t *testing.T) {
	t.Parallel()

	globalSettings := &mock.GlobalSettingsHandlerStub{}
	transferFunc := createNftTransferWithMockArguments(0, 1, globalSettings)
	_ = transferFunc.SetPayableChecker(&mock.PayableHandlerStub{})

	senderAddress := bytes.Repeat([]byte{2}, 32) // sender is in the same shard
	destinationAddress := bytes.Repeat([]byte{1}, 32)
	destinationAddress[31] = 0
	sender, err := transferFunc.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, transferFunc.marshaller, sender.(vmcommon.UserAccountHandler))

	_ = transferFunc.accounts.SaveAccount(sender)
	_, _ = transferFunc.accounts.Commit()
	// reload sender account
	sender, err = transferFunc.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}

	destination, _ := transferFunc.accounts.LoadAccount(destinationAddress)
	globalSettings.IsLimiterTransferCalled = func(token []byte) bool {
		return true
	}
	_, err = transferFunc.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	assert.Equal(t, ErrActionNotAllowed, err)

	vmInput.ReturnCallAfterError = true
	_, err = transferFunc.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	assert.Nil(t, err)
}

func TestDCTNFTTransfer_NotEnoughGas(t *testing.T) {
	t.Parallel()

	transferFunc := createNftTransferWithMockArguments(0, 1, &mock.GlobalSettingsHandlerStub{})
	_ = transferFunc.SetPayableChecker(&mock.PayableHandlerStub{})

	senderAddress := bytes.Repeat([]byte{2}, 32) // sender is in the same shard
	destinationAddress := bytes.Repeat([]byte{1}, 32)
	sender, err := transferFunc.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, transferFunc.marshaller, sender.(vmcommon.UserAccountHandler))
	_ = transferFunc.accounts.SaveAccount(sender)
	_, _ = transferFunc.accounts.Commit()
	// reload sender account
	sender, err = transferFunc.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 0,
		},
		RecipientAddr: senderAddress,
	}

	_, err = transferFunc.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), sender.(vmcommon.UserAccountHandler), vmInput)
	assert.Equal(t, err, ErrNotEnoughGas)
}

func extractScResultsFromVmOutput(t testing.TB, vmOutput *vmcommon.VMOutput) (string, [][]byte) {
	require.NotNil(t, vmOutput)
	require.Equal(t, 1, len(vmOutput.OutputAccounts))
	var outputAccount *vmcommon.OutputAccount
	for _, account := range vmOutput.OutputAccounts {
		outputAccount = account
		break
	}
	require.NotNil(t, outputAccount)
	if outputAccount == nil {
		// suppress next warnings, goland does not know about require.NotNil
		return "", nil
	}
	require.Equal(t, 1, len(outputAccount.OutputTransfers))
	outputTransfer := outputAccount.OutputTransfers[0]
	split := strings.Split(string(outputTransfer.Data), "@")

	args := make([][]byte, len(split)-1)
	var err error
	for i, splitArg := range split[1:] {
		args[i], err = hex.DecodeString(splitArg)
		require.Nil(t, err)
	}

	return split[0], args
}

func TestDCTNFTTransfer_SndDstFreezeCollection(t *testing.T) {
	t.Parallel()

	globalSettings := &mock.GlobalSettingsHandlerStub{}
	enableEpochsHandler := &mock.EnableEpochsHandlerStub{
		IsTransferToMetaFlagEnabledField:        true,
		IsCheckTransferFlagEnabledField:         true,
		IsCheckFrozenCollectionFlagEnabledField: true,
	}
	transferFunc, _ := createNFTTransferAndStorageHandler(0, 1, globalSettings, enableEpochsHandler)

	_ = transferFunc.SetPayableChecker(&mock.PayableHandlerStub{})

	senderAddress := bytes.Repeat([]byte{2}, 32) // sender is in the same shard
	destinationAddress := bytes.Repeat([]byte{1}, 32)
	destinationAddress[31] = 0
	sender, err := transferFunc.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, transferFunc.marshaller, sender.(vmcommon.UserAccountHandler))
	dctFrozen := DCTUserMetadata{Frozen: true}

	_ = transferFunc.accounts.SaveAccount(sender)
	_, _ = transferFunc.accounts.Commit()
	// reload sender account
	sender, err = transferFunc.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}

	destination, _ := transferFunc.accounts.LoadAccount(destinationAddress)
	tokenId := append(keyPrefix, tokenName...)
	dctToken := &dct.DCToken{Value: big.NewInt(0), Properties: dctFrozen.ToBytes()}
	marshaledData, _ := transferFunc.marshaller.Marshal(dctToken)
	_ = destination.(vmcommon.UserAccountHandler).AccountDataHandler().SaveKeyValue(tokenId, marshaledData)
	_ = transferFunc.accounts.SaveAccount(destination)
	_, _ = transferFunc.accounts.Commit()

	_, err = transferFunc.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	assert.Equal(t, ErrDCTIsFrozenForAccount, err)

	vmInput.ReturnCallAfterError = true
	_, err = transferFunc.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), destination.(vmcommon.UserAccountHandler), vmInput)
	assert.Nil(t, err)
}

func TestDctNFTTransfer_ProcessBuiltinFunctionCrossShardsFixOldLiquidityIssue(t *testing.T) {
	t.Parallel()

	vmInput, sender, nftTransferSenderShard, dctDataStorageHandler, tokenName, tokenNonce := createSetupToSendNFTCrossShard(t)

	dctDataStorageHandler.enableEpochsHandler.(*mock.EnableEpochsHandlerStub).IsFixOldTokenLiquidityEnabledField = true
	vmOutput, err := nftTransferSenderShard.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), nil, vmInput)
	require.Nil(t, err)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)

	_ = nftTransferSenderShard.accounts.SaveAccount(sender)
	_, _ = nftTransferSenderShard.accounts.Commit()

	// reload sender account
	sender, err = nftTransferSenderShard.accounts.LoadAccount(sender.AddressBytes())
	require.Nil(t, err)

	testNFTTokenShouldExist(t, nftTransferSenderShard.marshaller, sender, tokenName, tokenNonce, big.NewInt(2)) // 3 initial - 1 transferred
}

func TestDctNFTTransfer_ProcessBuiltinFunctionCrossShardsFixOldLiquidityIssueWithoutActivation(t *testing.T) {
	t.Parallel()

	vmInput, sender, nftTransferSenderShard, dctDataStorageHandler, _, _ := createSetupToSendNFTCrossShard(t)

	dctDataStorageHandler.enableEpochsHandler.(*mock.EnableEpochsHandlerStub).IsFixOldTokenLiquidityEnabledField = false
	_, err := nftTransferSenderShard.ProcessBuiltinFunction(sender.(vmcommon.UserAccountHandler), nil, vmInput)
	require.Equal(t, err, ErrInvalidLiquidityForDCT)
}

func createSetupToSendNFTCrossShard(t *testing.T) (*vmcommon.ContractCallInput, vmcommon.AccountHandler, *dctNFTTransfer, *dctDataStorage, []byte, uint64) {
	payableHandler := &mock.PayableHandlerStub{
		IsPayableCalled: func(address []byte) (bool, error) {
			return true, nil
		},
	}

	var enableEpochsHandler = &mock.EnableEpochsHandlerStub{
		IsSendAlwaysFlagEnabledField:            true,
		IsSaveToSystemAccountFlagEnabledField:   true,
		IsCheckFrozenCollectionFlagEnabledField: true,
	}
	nftTransferSenderShard, dctDataStorageHandler := createNFTTransferAndStorageHandler(1, 2, &mock.GlobalSettingsHandlerStub{}, enableEpochsHandler)
	_ = nftTransferSenderShard.SetPayableChecker(payableHandler)

	senderAddress := bytes.Repeat([]byte{1}, 32)
	destinationAddress := bytes.Repeat([]byte{2}, 32)
	sender, err := nftTransferSenderShard.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	tokenName := []byte("token")
	tokenNonce := uint64(1)

	initialTokens := big.NewInt(3)
	createDCTNFTToken(tokenName, core.NonFungible, tokenNonce, initialTokens, nftTransferSenderShard.marshaller, sender.(vmcommon.UserAccountHandler))
	_ = nftTransferSenderShard.accounts.SaveAccount(sender)
	_, _ = nftTransferSenderShard.accounts.Commit()

	// reload sender account
	sender, err = nftTransferSenderShard.accounts.LoadAccount(senderAddress)
	require.Nil(t, err)

	nonceBytes := big.NewInt(int64(tokenNonce)).Bytes()
	quantityBytes := big.NewInt(1).Bytes()
	scCallFunctionAsHex := hex.EncodeToString([]byte("functionToCall"))
	scCallArg := hex.EncodeToString([]byte("arg"))
	scCallArgs := [][]byte{[]byte(scCallFunctionAsHex), []byte(scCallArg)}
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			CallerAddr:  senderAddress,
			Arguments:   [][]byte{tokenName, nonceBytes, quantityBytes, destinationAddress},
			GasProvided: 1,
		},
		RecipientAddr: senderAddress,
	}
	vmInput.Arguments = append(vmInput.Arguments, scCallArgs...)

	return vmInput, sender, nftTransferSenderShard, dctDataStorageHandler, tokenName, tokenNonce
}
