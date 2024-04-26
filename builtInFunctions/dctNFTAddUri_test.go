package builtInFunctions

import (
	"errors"
	"math/big"
	"testing"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/mock"
	"github.com/stretchr/testify/require"
)

func TestNewDCTNFTAddUriFunc(t *testing.T) {
	t.Parallel()

	// nil marshalizer
	e, err := NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, nil, nil, nil, 0, &mock.EpochNotifierStub{})
	require.True(t, check.IfNil(e))
	require.Equal(t, ErrNilDCTNFTStorageHandler, err)

	// nil pause handler
	e, err = NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), nil, nil, 0, &mock.EpochNotifierStub{})
	require.True(t, check.IfNil(e))
	require.Equal(t, ErrNilGlobalSettingsHandler, err)

	// nil roles handler
	e, err = NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, nil, 0, &mock.EpochNotifierStub{})
	require.True(t, check.IfNil(e))
	require.Equal(t, ErrNilRolesHandler, err)

	// nil epoch notifier
	e, err = NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 0, nil)
	require.True(t, check.IfNil(e))
	require.Equal(t, ErrNilEpochHandler, err)

	// should work
	e, err = NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 1, &mock.EpochNotifierStub{})
	require.False(t, check.IfNil(e))
	require.NoError(t, err)
	require.False(t, e.IsActive())
}

func TestDCTNFTAddUri_SetNewGasConfig_NilGasCost(t *testing.T) {
	t.Parallel()

	defaultGasCost := uint64(10)
	e, _ := NewDCTNFTAddUriFunc(defaultGasCost, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 0, &mock.EpochNotifierStub{})

	e.SetNewGasConfig(nil)
	require.Equal(t, defaultGasCost, e.funcGasCost)
}

func TestDCTNFTAddUri_SetNewGasConfig_ShouldWork(t *testing.T) {
	t.Parallel()

	defaultGasCost := uint64(10)
	newGasCost := uint64(37)
	e, _ := NewDCTNFTAddUriFunc(defaultGasCost, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 0, &mock.EpochNotifierStub{})

	e.SetNewGasConfig(
		&vmcommon.GasCost{
			BuiltInCost: vmcommon.BuiltInCost{
				DCTNFTAddURI: newGasCost,
			},
		},
	)

	require.Equal(t, newGasCost, e.funcGasCost)
}

func TestDCTNFTAddUri_ProcessBuiltinFunctionErrorOnCheckInput(t *testing.T) {
	t.Parallel()

	e, _ := NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 0, &mock.EpochNotifierStub{})

	// nil vm input
	output, err := e.ProcessBuiltinFunction(mock.NewAccountWrapMock([]byte("addr")), nil, nil)
	require.Nil(t, output)
	require.Equal(t, ErrNilVmInput, err)

	// vm input - value not zero
	output, err = e.ProcessBuiltinFunction(
		mock.NewAccountWrapMock([]byte("addr")),
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue: big.NewInt(37),
			},
		},
	)
	require.Nil(t, output)
	require.Equal(t, ErrBuiltInFunctionCalledWithValue, err)

	// vm input - invalid number of arguments
	output, err = e.ProcessBuiltinFunction(
		mock.NewAccountWrapMock([]byte("addr")),
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue: big.NewInt(0),
				Arguments: [][]byte{[]byte("single arg")},
			},
		},
	)
	require.Nil(t, output)
	require.Equal(t, ErrInvalidArguments, err)

	// vm input - invalid number of arguments
	output, err = e.ProcessBuiltinFunction(
		mock.NewAccountWrapMock([]byte("addr")),
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue: big.NewInt(0),
				Arguments: [][]byte{[]byte("arg0")},
			},
		},
	)
	require.Nil(t, output)
	require.Equal(t, ErrInvalidArguments, err)

	// vm input - invalid receiver
	output, err = e.ProcessBuiltinFunction(
		mock.NewAccountWrapMock([]byte("addr")),
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:  big.NewInt(0),
				Arguments:  [][]byte{[]byte("arg0"), []byte("arg1")},
				CallerAddr: []byte("address 1"),
			},
			RecipientAddr: []byte("address 2"),
		},
	)
	require.Nil(t, output)
	require.Equal(t, ErrInvalidRcvAddr, err)

	// nil user account
	output, err = e.ProcessBuiltinFunction(
		nil,
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:  big.NewInt(0),
				Arguments:  [][]byte{[]byte("arg0"), []byte("arg1")},
				CallerAddr: []byte("address 1"),
			},
			RecipientAddr: []byte("address 1"),
		},
	)
	require.Nil(t, output)
	require.Equal(t, ErrNilUserAccount, err)

	// not enough gas
	output, err = e.ProcessBuiltinFunction(
		mock.NewAccountWrapMock([]byte("addr")),
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:   big.NewInt(0),
				Arguments:   [][]byte{[]byte("arg0"), []byte("arg1")},
				CallerAddr:  []byte("address 1"),
				GasProvided: 1,
			},
			RecipientAddr: []byte("address 1"),
		},
	)
	require.Nil(t, output)
	require.Equal(t, ErrNotEnoughGas, err)
}

func TestDCTNFTAddUri_ProcessBuiltinFunctionInvalidNumberOfArguments(t *testing.T) {
	t.Parallel()

	e, _ := NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 0, &mock.EpochNotifierStub{})
	output, err := e.ProcessBuiltinFunction(
		mock.NewAccountWrapMock([]byte("addr")),
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:   big.NewInt(0),
				Arguments:   [][]byte{[]byte("arg0"), []byte("arg1")},
				CallerAddr:  []byte("address 1"),
				GasProvided: 12,
			},
			RecipientAddr: []byte("address 1"),
		},
	)
	require.Nil(t, output)
	require.Equal(t, ErrInvalidArguments, err)
}

func TestDCTNFTAddUri_ProcessBuiltinFunctionCheckAllowedToExecuteError(t *testing.T) {
	t.Parallel()

	localErr := errors.New("err")
	rolesHandler := &mock.DCTRoleHandlerStub{
		CheckAllowedToExecuteCalled: func(_ vmcommon.UserAccountHandler, _ []byte, _ []byte) error {
			return localErr
		},
	}
	e, _ := NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, rolesHandler, 0, &mock.EpochNotifierStub{})
	output, err := e.ProcessBuiltinFunction(
		mock.NewAccountWrapMock([]byte("addr")),
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:   big.NewInt(0),
				Arguments:   [][]byte{[]byte("arg0"), []byte("arg1"), []byte("arg2")},
				CallerAddr:  []byte("address 1"),
				GasProvided: 12,
			},
			RecipientAddr: []byte("address 1"),
		},
	)

	require.Nil(t, output)
	require.Equal(t, localErr, err)
}

func TestDCTNFTAddUri_ProcessBuiltinFunctionNewSenderShouldErr(t *testing.T) {
	t.Parallel()

	e, _ := NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 0, &mock.EpochNotifierStub{})
	output, err := e.ProcessBuiltinFunction(
		mock.NewAccountWrapMock([]byte("addr")),
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:   big.NewInt(0),
				Arguments:   [][]byte{[]byte("arg0"), []byte("arg1"), []byte("arg2")},
				CallerAddr:  []byte("address 1"),
				GasProvided: 12,
			},
			RecipientAddr: []byte("address 1"),
		},
	)

	require.Nil(t, output)
	require.Error(t, err)
	require.Equal(t, ErrNewNFTDataOnSenderAddress, err)
}

func TestDCTNFTAddUri_ProcessBuiltinFunctionMetaDataMissing(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	e, _ := NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandler(), &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 0, &mock.EpochNotifierStub{})

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{}
	dctDataBytes, _ := marshalizer.Marshal(dctData)
	_ = userAcc.AccountDataHandler().SaveKeyValue([]byte(core.NumbatProtectedKeyPrefix+core.DCTKeyIdentifier+"arg0"+"arg1"), dctDataBytes)
	output, err := e.ProcessBuiltinFunction(
		userAcc,
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:   big.NewInt(0),
				Arguments:   [][]byte{[]byte("arg0"), {0}, []byte("arg2")},
				CallerAddr:  []byte("address 1"),
				GasProvided: 12,
			},
			RecipientAddr: []byte("address 1"),
		},
	)

	require.Nil(t, output)
	require.Equal(t, ErrNFTDoesNotHaveMetadata, err)
}

func TestDCTNFTAddUri_ProcessBuiltinFunctionShouldErrOnSaveBecauseTokenIsPaused(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	globalSettingsHandler := &mock.GlobalSettingsHandlerStub{
		IsPausedCalled: func(_ []byte) bool {
			return true
		},
	}

	e, _ := NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, createNewDCTDataStorageHandlerWithArgs(globalSettingsHandler, &mock.AccountsStub{}), globalSettingsHandler, &mock.DCTRoleHandlerStub{}, 0, &mock.EpochNotifierStub{})

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{
		TokenMetaData: &dct.MetaData{
			Name: []byte("test"),
		},
		Value: big.NewInt(10),
	}
	dctDataBytes, _ := marshalizer.Marshal(dctData)
	_ = userAcc.AccountDataHandler().SaveKeyValue([]byte(core.NumbatProtectedKeyPrefix+core.DCTKeyIdentifier+"arg0"+"arg1"), dctDataBytes)

	output, err := e.ProcessBuiltinFunction(
		userAcc,
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:   big.NewInt(0),
				Arguments:   [][]byte{[]byte("arg0"), []byte("arg1"), []byte("arg2")},
				CallerAddr:  []byte("address 1"),
				GasProvided: 12,
			},
			RecipientAddr: []byte("address 1"),
		},
	)

	require.Nil(t, output)
	require.Equal(t, ErrDCTTokenIsPaused, err)
}

func TestDCTNFTAddUri_ProcessBuiltinFunctionShouldWork(t *testing.T) {
	t.Parallel()

	tokenIdentifier := "testTkn"
	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + tokenIdentifier

	nonce := big.NewInt(33)
	initialValue := big.NewInt(5)
	URIToAdd := []byte("NewURI")

	dctDataStorage := createNewDCTDataStorageHandler()
	marshalizer := &mock.MarshalizerMock{}
	e, _ := NewDCTNFTAddUriFunc(10, vmcommon.BaseOperationCost{}, dctDataStorage, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}, 0, &mock.EpochNotifierStub{})

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{
		TokenMetaData: &dct.MetaData{
			Name: []byte("test"),
		},
		Value: initialValue,
	}
	dctDataBytes, _ := marshalizer.Marshal(dctData)
	tokenKey := append([]byte(key), nonce.Bytes()...)
	_ = userAcc.AccountDataHandler().SaveKeyValue(tokenKey, dctDataBytes)

	output, err := e.ProcessBuiltinFunction(
		userAcc,
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:   big.NewInt(0),
				Arguments:   [][]byte{[]byte(tokenIdentifier), nonce.Bytes(), URIToAdd},
				CallerAddr:  []byte("address 1"),
				GasProvided: 12,
			},
			RecipientAddr: []byte("address 1"),
		},
	)

	require.NotNil(t, output)
	require.NoError(t, err)
	require.Equal(t, vmcommon.Ok, output.ReturnCode)

	res, err := userAcc.AccountDataHandler().RetrieveValue(tokenKey)
	require.NoError(t, err)
	require.NotNil(t, res)

	metaData, _ := dctDataStorage.getDCTMetaDataFromSystemAccount(tokenKey)
	require.Equal(t, metaData.URIs[0], URIToAdd)
}
