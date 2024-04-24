package builtInFunctions

import (
	"errors"
	"math/big"
	"testing"

	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/data/dct"
	"github.com/numbatx/gn-vm-common/mock"
	"github.com/stretchr/testify/require"
)

func TestNewDCTRolesFunc_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	dctRolesF, err := NewDCTRolesFunc(nil, false)

	require.Equal(t, ErrNilMarshalizer, err)
	require.Nil(t, dctRolesF)
}

func TestDctRoles_ProcessBuiltinFunction_NilVMInputShouldErr(t *testing.T) {
	t.Parallel()

	dctRolesF, _ := NewDCTRolesFunc(nil, false)

	_, err := dctRolesF.ProcessBuiltinFunction(nil, &mock.UserAccountStub{}, nil)
	require.Equal(t, ErrNilVmInput, err)
}

func TestDctRoles_ProcessBuiltinFunction_WrongCalledShouldErr(t *testing.T) {
	t.Parallel()

	dctRolesF, _ := NewDCTRolesFunc(nil, false)

	_, err := dctRolesF.ProcessBuiltinFunction(nil, &mock.UserAccountStub{}, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: []byte{},
			Arguments:  [][]byte{[]byte("1"), []byte("2")},
		},
	})
	require.Equal(t, ErrAddressIsNotDCTSystemSC, err)
}

func TestDctRoles_ProcessBuiltinFunction_NilAccountDestShouldErr(t *testing.T) {
	t.Parallel()

	dctRolesF, _ := NewDCTRolesFunc(nil, false)

	_, err := dctRolesF.ProcessBuiltinFunction(nil, nil, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: vmcommon.DCTSCAddress,
			Arguments:  [][]byte{[]byte("1"), []byte("2")},
		},
	})
	require.Equal(t, ErrNilUserAccount, err)
}

func TestDctRoles_ProcessBuiltinFunction_GetRolesFailShouldErr(t *testing.T) {
	t.Parallel()

	dctRolesF, _ := NewDCTRolesFunc(&mock.MarshalizerMock{Fail: true}, false)

	_, err := dctRolesF.ProcessBuiltinFunction(nil, &mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					return nil, nil
				},
			}
		},
	}, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: vmcommon.DCTSCAddress,
			Arguments:  [][]byte{[]byte("1"), []byte("2")},
		},
	})
	require.Error(t, err)
}

func TestDctRoles_ProcessBuiltinFunction_GetRolesFailShouldWorkEvenIfAccntTrieIsNil(t *testing.T) {
	t.Parallel()

	saveKeyWasCalled := false
	dctRolesF, _ := NewDCTRolesFunc(&mock.MarshalizerMock{}, false)

	_, err := dctRolesF.ProcessBuiltinFunction(nil, &mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(_ []byte) ([]byte, error) {
					return nil, nil
				},
				SaveKeyValueCalled: func(_ []byte, _ []byte) error {
					saveKeyWasCalled = true
					return nil
				},
			}
		},
	}, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: vmcommon.DCTSCAddress,
			Arguments:  [][]byte{[]byte("1"), []byte("2")},
		},
	})
	require.NoError(t, err)
	require.True(t, saveKeyWasCalled)
}

func TestDctRoles_ProcessBuiltinFunction_SetRolesShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, true)

	acc := &mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					roles := &dct.DCTRoles{}
					return marshalizer.Marshal(roles)
				},
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					roles := &dct.DCTRoles{}
					_ = marshalizer.Unmarshal(roles, value)
					require.Equal(t, roles.Roles, [][]byte{[]byte(vmcommon.DCTRoleLocalMint)})
					return nil
				},
			}
		},
	}
	_, err := dctRolesF.ProcessBuiltinFunction(nil, acc, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: vmcommon.DCTSCAddress,
			Arguments:  [][]byte{[]byte("1"), []byte(vmcommon.DCTRoleLocalMint)},
		},
	})
	require.Nil(t, err)
}

func TestDctRoles_ProcessBuiltinFunction_SaveFailedShouldErr(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, true)

	localErr := errors.New("local err")
	acc := &mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					roles := &dct.DCTRoles{}
					return marshalizer.Marshal(roles)
				},
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return localErr
				},
			}
		},
	}
	_, err := dctRolesF.ProcessBuiltinFunction(nil, acc, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: vmcommon.DCTSCAddress,
			Arguments:  [][]byte{[]byte("1"), []byte(vmcommon.DCTRoleLocalMint)},
		},
	})
	require.Equal(t, localErr, err)
}

func TestDctRoles_ProcessBuiltinFunction_UnsetRolesDoesNotExistsShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, false)

	acc := &mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					roles := &dct.DCTRoles{}
					return marshalizer.Marshal(roles)
				},
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					roles := &dct.DCTRoles{}
					_ = marshalizer.Unmarshal(roles, value)
					require.Len(t, roles.Roles, 0)
					return nil
				},
			}
		},
	}
	_, err := dctRolesF.ProcessBuiltinFunction(nil, acc, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: vmcommon.DCTSCAddress,
			Arguments:  [][]byte{[]byte("1"), []byte(vmcommon.DCTRoleLocalMint)},
		},
	})
	require.Nil(t, err)
}

func TestDctRoles_ProcessBuiltinFunction_UnsetRolesShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, false)

	acc := &mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					roles := &dct.DCTRoles{
						Roles: [][]byte{[]byte(vmcommon.DCTRoleLocalMint)},
					}
					return marshalizer.Marshal(roles)
				},
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					roles := &dct.DCTRoles{}
					_ = marshalizer.Unmarshal(roles, value)
					require.Len(t, roles.Roles, 0)
					return nil
				},
			}
		},
	}
	_, err := dctRolesF.ProcessBuiltinFunction(nil, acc, &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue:  big.NewInt(0),
			CallerAddr: vmcommon.DCTSCAddress,
			Arguments:  [][]byte{[]byte("1"), []byte(vmcommon.DCTRoleLocalMint)},
		},
	})
	require.Nil(t, err)
}

func TestDctRoles_CheckAllowedToExecuteNilAccountShouldErr(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, false)

	err := dctRolesF.CheckAllowedToExecute(nil, []byte("ID"), []byte(vmcommon.DCTRoleLocalBurn))
	require.Equal(t, ErrNilUserAccount, err)
}

func TestDctRoles_CheckAllowedToExecuteCannotGetDCTRole(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{Fail: true}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, false)

	err := dctRolesF.CheckAllowedToExecute(&mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					return nil, nil
				},
			}
		},
	}, []byte("ID"), []byte(vmcommon.DCTRoleLocalBurn))
	require.Error(t, err)
}

func TestDctRoles_CheckAllowedToExecuteIsNewNotAllowed(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, false)

	err := dctRolesF.CheckAllowedToExecute(&mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					return nil, nil
				},
			}
		},
	}, []byte("ID"), []byte(vmcommon.DCTRoleLocalBurn))
	require.Equal(t, ErrActionNotAllowed, err)
}

func TestDctRoles_CheckAllowed_ShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, false)

	err := dctRolesF.CheckAllowedToExecute(&mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					roles := &dct.DCTRoles{
						Roles: [][]byte{[]byte(vmcommon.DCTRoleLocalMint)},
					}
					return marshalizer.Marshal(roles)
				},
			}
		},
	}, []byte("ID"), []byte(vmcommon.DCTRoleLocalMint))
	require.Nil(t, err)
}

func TestDctRoles_CheckAllowedToExecuteRoleNotFind(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	dctRolesF, _ := NewDCTRolesFunc(marshalizer, false)

	err := dctRolesF.CheckAllowedToExecute(&mock.UserAccountStub{
		AccountDataHandlerCalled: func() vmcommon.AccountDataHandler {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(key []byte) ([]byte, error) {
					roles := &dct.DCTRoles{
						Roles: [][]byte{[]byte(vmcommon.DCTRoleLocalBurn)},
					}
					return marshalizer.Marshal(roles)
				},
			}
		},
	}, []byte("ID"), []byte(vmcommon.DCTRoleLocalMint))
	require.Equal(t, ErrActionNotAllowed, err)
}
