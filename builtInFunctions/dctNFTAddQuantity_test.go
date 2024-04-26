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

func TestNewDCTNFTAddQuantityFunc(t *testing.T) {
	t.Parallel()

	// nil marshalizer
	eqf, err := NewDCTNFTAddQuantityFunc(10, nil, nil, nil)
	require.True(t, check.IfNil(eqf))
	require.Equal(t, ErrNilMarshalizer, err)

	// nil pause handler
	eqf, err = NewDCTNFTAddQuantityFunc(10, &mock.MarshalizerMock{}, nil, nil)
	require.True(t, check.IfNil(eqf))
	require.Equal(t, ErrNilGlobalSettingsHandler, err)

	// nil roles handler
	eqf, err = NewDCTNFTAddQuantityFunc(10, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, nil)
	require.True(t, check.IfNil(eqf))
	require.Equal(t, ErrNilRolesHandler, err)

	// should work
	eqf, err = NewDCTNFTAddQuantityFunc(10, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})
	require.False(t, check.IfNil(eqf))
	require.NoError(t, err)
}

func TestDctNFTAddQuantity_SetNewGasConfig_NilGasCost(t *testing.T) {
	t.Parallel()

	defaultGasCost := uint64(10)
	eqf, _ := NewDCTNFTAddQuantityFunc(defaultGasCost, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})

	eqf.SetNewGasConfig(nil)
	require.Equal(t, defaultGasCost, eqf.funcGasCost)
}

func TestDctNFTAddQuantity_SetNewGasConfig_ShouldWork(t *testing.T) {
	t.Parallel()

	defaultGasCost := uint64(10)
	newGasCost := uint64(37)
	eqf, _ := NewDCTNFTAddQuantityFunc(defaultGasCost, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})

	eqf.SetNewGasConfig(
		&vmcommon.GasCost{
			BuiltInCost: vmcommon.BuiltInCost{
				DCTNFTAddQuantity: newGasCost,
			},
		},
	)

	require.Equal(t, newGasCost, eqf.funcGasCost)
}

func TestDctNFTAddQuantity_ProcessBuiltinFunctionErrorOnCheckDCTNFTCreateBurnAddInput(t *testing.T) {
	t.Parallel()

	eqf, _ := NewDCTNFTAddQuantityFunc(10, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})

	// nil vm input
	output, err := eqf.ProcessBuiltinFunction(mock.NewAccountWrapMock([]byte("addr")), nil, nil)
	require.Nil(t, output)
	require.Equal(t, ErrNilVmInput, err)

	// vm input - value not zero
	output, err = eqf.ProcessBuiltinFunction(
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
	output, err = eqf.ProcessBuiltinFunction(
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
	output, err = eqf.ProcessBuiltinFunction(
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
	output, err = eqf.ProcessBuiltinFunction(
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
	output, err = eqf.ProcessBuiltinFunction(
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
	output, err = eqf.ProcessBuiltinFunction(
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

func TestDctNFTAddQuantity_ProcessBuiltinFunctionInvalidNumberOfArguments(t *testing.T) {
	t.Parallel()

	eqf, _ := NewDCTNFTAddQuantityFunc(10, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})
	output, err := eqf.ProcessBuiltinFunction(
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

func TestDctNFTAddQuantity_ProcessBuiltinFunctionCheckAllowedToExecuteError(t *testing.T) {
	t.Parallel()

	localErr := errors.New("err")
	rolesHandler := &mock.DCTRoleHandlerStub{
		CheckAllowedToExecuteCalled: func(_ vmcommon.UserAccountHandler, _ []byte, _ []byte) error {
			return localErr
		},
	}
	eqf, _ := NewDCTNFTAddQuantityFunc(10, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, rolesHandler)
	output, err := eqf.ProcessBuiltinFunction(
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

func TestDctNFTAddQuantity_ProcessBuiltinFunctionNewSenderShouldErr(t *testing.T) {
	t.Parallel()

	eqf, _ := NewDCTNFTAddQuantityFunc(10, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})
	output, err := eqf.ProcessBuiltinFunction(
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

func TestDctNFTAddQuantity_ProcessBuiltinFunctionMetaDataMissing(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	eqf, _ := NewDCTNFTAddQuantityFunc(10, marshalizer, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{}
	dctDataBytes, _ := marshalizer.Marshal(dctData)
	_ = userAcc.AccountDataHandler().SaveKeyValue([]byte(core.NumbatProtectedKeyPrefix+core.DCTKeyIdentifier+"arg0"), dctDataBytes)
	output, err := eqf.ProcessBuiltinFunction(
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

func TestDctNFTAddQuantity_ProcessBuiltinFunctionShouldErrOnSaveBecauseTokenIsPaused(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	globalSettingsHandler := &mock.GlobalSettingsHandlerStub{
		IsPausedCalled: func(_ []byte) bool {
			return true
		},
	}

	eqf, _ := NewDCTNFTAddQuantityFunc(10, marshalizer, globalSettingsHandler, &mock.DCTRoleHandlerStub{})

	userAcc := mock.NewAccountWrapMock([]byte("addr"))
	dctData := &dct.DCToken{
		TokenMetaData: &dct.MetaData{
			Name: []byte("test"),
		},
		Value: big.NewInt(10),
	}
	dctDataBytes, _ := marshalizer.Marshal(dctData)
	_ = userAcc.AccountDataHandler().SaveKeyValue([]byte(core.NumbatProtectedKeyPrefix+core.DCTKeyIdentifier+"arg0"+"arg1"), dctDataBytes)

	output, err := eqf.ProcessBuiltinFunction(
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

func TestDctNFTAddQuantity_ProcessBuiltinFunctionShouldWork(t *testing.T) {
	t.Parallel()

	tokenIdentifier := "testTkn"
	key := core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier + tokenIdentifier

	nonce := big.NewInt(33)
	initialValue := big.NewInt(5)
	valueToAdd := big.NewInt(37)
	expectedValue := big.NewInt(0).Add(initialValue, valueToAdd)

	marshalizer := &mock.MarshalizerMock{}
	eqf, _ := NewDCTNFTAddQuantityFunc(10, marshalizer, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})

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

	output, err := eqf.ProcessBuiltinFunction(
		userAcc,
		nil,
		&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				CallValue:   big.NewInt(0),
				Arguments:   [][]byte{[]byte(tokenIdentifier), nonce.Bytes(), valueToAdd.Bytes()},
				CallerAddr:  []byte("address 1"),
				GasProvided: 12,
			},
			RecipientAddr: []byte("address 1"),
		},
	)

	require.NotNil(t, output)
	require.NoError(t, err)
	require.Equal(t, vmcommon.Ok, output.ReturnCode)

	res, err := userAcc.AccountDataHandler().RetrieveValue([]byte(key))
	require.NoError(t, err)
	require.NotNil(t, res)

	finalTokenData := dct.DCToken{}
	_ = marshalizer.Unmarshal(&finalTokenData, res)
	require.Equal(t, expectedValue.Bytes(), finalTokenData.Value.Bytes())
}
