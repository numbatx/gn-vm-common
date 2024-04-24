package builtInFunctions

import (
	"bytes"

	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/check"
)

type dctPause struct {
	baseAlwaysActive
	keyPrefix []byte
	pause     bool
	accounts  vmcommon.AccountsAdapter
}

// NewDCTPauseFunc returns the dct pause/un-pause built-in function component
func NewDCTPauseFunc(
	accounts vmcommon.AccountsAdapter,
	pause bool,
) (*dctPause, error) {
	if check.IfNil(accounts) {
		return nil, ErrNilAccountsAdapter
	}

	e := &dctPause{
		keyPrefix: []byte(vmcommon.NumbatProtectedKeyPrefix + vmcommon.DCTKeyIdentifier),
		pause:     pause,
		accounts:  accounts,
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctPause) SetNewGasConfig(_ *vmcommon.GasCost) {
}

// ProcessBuiltinFunction resolves DCT pause function call
func (e *dctPause) ProcessBuiltinFunction(
	_, _ vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	if vmInput == nil {
		return nil, ErrNilVmInput
	}
	if vmInput.CallValue.Cmp(zero) != 0 {
		return nil, ErrBuiltInFunctionCalledWithValue
	}
	if len(vmInput.Arguments) != 1 {
		return nil, ErrInvalidArguments
	}
	if !bytes.Equal(vmInput.CallerAddr, vmcommon.DCTSCAddress) {
		return nil, ErrAddressIsNotDCTSystemSC
	}
	if !vmcommon.IsSystemAccountAddress(vmInput.RecipientAddr) {
		return nil, ErrOnlySystemAccountAccepted
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)

	err := e.togglePause(dctTokenKey)
	if err != nil {
		return nil, err
	}

	vmOutput := &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}
	return vmOutput, nil
}

func (e *dctPause) togglePause(token []byte) error {
	systemSCAccount, err := e.getSystemAccount()
	if err != nil {
		return err
	}

	val, _ := systemSCAccount.AccountDataHandler().RetrieveValue(token)
	dctMetaData := DCTGlobalMetadataFromBytes(val)
	dctMetaData.Paused = e.pause
	err = systemSCAccount.AccountDataHandler().SaveKeyValue(token, dctMetaData.ToBytes())
	if err != nil {
		return err
	}

	return e.accounts.SaveAccount(systemSCAccount)
}

func (e *dctPause) getSystemAccount() (vmcommon.UserAccountHandler, error) {
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

// IsPaused returns true if the token is paused
func (e *dctPause) IsPaused(pauseKey []byte) bool {
	systemSCAccount, err := e.getSystemAccount()
	if err != nil {
		return false
	}

	val, _ := systemSCAccount.AccountDataHandler().RetrieveValue(pauseKey)
	if len(val) != lengthOfDCTMetadata {
		return false
	}
	dctMetaData := DCTGlobalMetadataFromBytes(val)

	return dctMetaData.Paused
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctPause) IsInterfaceNil() bool {
	return e == nil
}
