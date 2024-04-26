package builtInFunctions

import (
	"errors"
	"math/big"
	"testing"

	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/mock"
	"github.com/stretchr/testify/require"
)

func TestNewDCTLocalBurnFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		argsFunc func() (c uint64, m vmcommon.Marshalizer, p vmcommon.DCTGlobalSettingsHandler, r vmcommon.DCTRoleHandler)
		exError  error
	}{
		{
			name: "NilMarshalizer",
			argsFunc: func() (c uint64, m vmcommon.Marshalizer, p vmcommon.DCTGlobalSettingsHandler, r vmcommon.DCTRoleHandler) {
				return 0, nil, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}
			},
			exError: ErrNilMarshalizer,
		},
		{
			name: "NilGlobalSettingsHandler",
			argsFunc: func() (c uint64, m vmcommon.Marshalizer, p vmcommon.DCTGlobalSettingsHandler, r vmcommon.DCTRoleHandler) {
				return 0, &mock.MarshalizerMock{}, nil, &mock.DCTRoleHandlerStub{}
			},
			exError: ErrNilGlobalSettingsHandler,
		},
		{
			name: "NilRolesHandler",
			argsFunc: func() (c uint64, m vmcommon.Marshalizer, p vmcommon.DCTGlobalSettingsHandler, r vmcommon.DCTRoleHandler) {
				return 0, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, nil
			},
			exError: ErrNilRolesHandler,
		},
		{
			name: "Ok",
			argsFunc: func() (c uint64, m vmcommon.Marshalizer, p vmcommon.DCTGlobalSettingsHandler, r vmcommon.DCTRoleHandler) {
				return 0, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{}
			},
			exError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDCTLocalBurnFunc(tt.argsFunc())
			require.Equal(t, err, tt.exError)
		})
	}
}

func TestDctLocalBurn_ProcessBuiltinFunction_CalledWithValueShouldErr(t *testing.T) {
	t.Parallel()

	dctLocalBurnF, _ := NewDCTLocalBurnFunc(0, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})

	_, err := dctLocalBurnF.ProcessBuiltinFunction(&mock.AccountWrapMock{}, &mock.AccountWrapMock{}, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue: big.NewInt(1),
		},
	})
	require.Equal(t, ErrBuiltInFunctionCalledWithValue, err)
}

func TestDctLocalBurn_ProcessBuiltinFunction_CheckAllowToExecuteShouldErr(t *testing.T) {
	t.Parallel()

	localErr := errors.New("local err")
	dctLocalBurnF, _ := NewDCTLocalBurnFunc(0, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{
		CheckAllowedToExecuteCalled: func(account vmcommon.UserAccountHandler, tokenID []byte, action []byte) error {
			return localErr
		},
	})

	_, err := dctLocalBurnF.ProcessBuiltinFunction(&mock.AccountWrapMock{}, &mock.AccountWrapMock{}, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue: big.NewInt(0),
			Arguments: [][]byte{[]byte("arg1"), []byte("arg2")},
		},
	})
	require.Equal(t, localErr, err)
}

func TestDctLocalBurn_ProcessBuiltinFunction_CannotAddToDctBalanceShouldErr(t *testing.T) {
	t.Parallel()

	dctLocalBurnF, _ := NewDCTLocalBurnFunc(0, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{
		CheckAllowedToExecuteCalled: func(account vmcommon.UserAccountHandler, tokenID []byte, action []byte) error {
			return nil
		},
	})

	localErr := errors.New("local err")
	_, err := dctLocalBurnF.ProcessBuiltinFunction(&mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					return nil, localErr
				},
			}
		},
	}, &mock.AccountWrapMock{}, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue: big.NewInt(0),
			Arguments: [][]byte{[]byte("arg1"), []byte("arg2")},
		},
	})
	require.Equal(t, ErrInsufficientFunds, err)
}

func TestDctLocalBurn_ProcessBuiltinFunction_ShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctLocalBurnF, _ := NewDCTLocalBurnFunc(50, marshalizer, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{
		CheckAllowedToExecuteCalled: func(account vmcommon.UserAccountHandler, tokenID []byte, action []byte) error {
			return nil
		},
	})

	sndAccout := &mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					dctData := &dct.DCToken{Value: big.NewInt(100)}
					return marshalizer.Marshal(dctData)
				},
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					dctData := &dct.DCToken{}
					_ = marshalizer.Unmarshal(dctData, value)
					require.Equal(t, big.NewInt(99), dctData.Value)
					return nil
				},
			}
		},
	}
	vmOutput, err := dctLocalBurnF.ProcessBuiltinFunction(sndAccout, &mock.AccountWrapMock{}, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			Arguments:   [][]byte{[]byte("arg1"), big.NewInt(1).Bytes()},
			GasProvided: 500,
		},
	})
	require.Equal(t, nil, err)

	expectedVMOutput := &vmcommon.VMOutput{
		ReturnCode:   vmcommon.Ok,
		GasRemaining: 450,
		Logs: []*vmcommon.LogEntry{
			{
				Identifier: []byte("DCTLocalBurn"),
				Address:    nil,
				Topics:     [][]byte{[]byte("arg1"), big.NewInt(0).Bytes(), big.NewInt(1).Bytes()},
				Data:       nil,
			},
		},
	}
	require.Equal(t, expectedVMOutput, vmOutput)
}

func TestDctLocalBurn_SetNewGasConfig(t *testing.T) {
	t.Parallel()

	dctLocalBurnF, _ := NewDCTLocalBurnFunc(0, &mock.MarshalizerMock{}, &mock.GlobalSettingsHandlerStub{}, &mock.DCTRoleHandlerStub{})

	dctLocalBurnF.SetNewGasConfig(&vmcommon.GasCost{BuiltInCost: vmcommon.BuiltInCost{
		DCTLocalBurn: 500},
	})

	require.Equal(t, uint64(500), dctLocalBurnF.funcGasCost)
}

func TestCheckInputArgumentsForLocalAction_InvalidRecipientAddr(t *testing.T) {
	t.Parallel()

	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			Arguments:  [][]byte{[]byte("arg1"), big.NewInt(1).Bytes()},
			CallerAddr: []byte("caller"),
		},
		RecipientAddr: []byte("rec"),
	}

	err := checkInputArgumentsForLocalAction(&mock.UserAccountStub{}, vmInput, 0)
	require.Equal(t, ErrInvalidRcvAddr, err)
}

func TestCheckInputArgumentsForLocalAction_NilUserAccount(t *testing.T) {
	t.Parallel()

	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			Arguments:  [][]byte{[]byte("arg1"), big.NewInt(1).Bytes()},
			CallerAddr: []byte("caller"),
		},
		RecipientAddr: []byte("caller"),
	}

	err := checkInputArgumentsForLocalAction(nil, vmInput, 0)
	require.Equal(t, ErrNilUserAccount, err)
}

func TestCheckInputArgumentsForLocalAction_NotEnoughGas(t *testing.T) {
	t.Parallel()

	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:   big.NewInt(0),
			Arguments:   [][]byte{[]byte("arg1"), big.NewInt(10).Bytes()},
			CallerAddr:  []byte("caller"),
			GasProvided: 1,
		},
		RecipientAddr: []byte("caller"),
	}

	err := checkInputArgumentsForLocalAction(&mock.UserAccountStub{}, vmInput, 500)
	require.Equal(t, ErrNotEnoughGas, err)
}
