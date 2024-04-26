package builtInFunctions

import (
	"bytes"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/atomic"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-vm-common"
)

type dctGlobalSettings struct {
	*baseEnabled
	keyPrefix []byte
	set       bool
	accounts  vmcommon.AccountsAdapter
}

// NewDCTGlobalSettingsFunc returns the dct pause/un-pause built-in function component
func NewDCTGlobalSettingsFunc(
	accounts vmcommon.AccountsAdapter,
	set bool,
	function string,
	activationEpoch uint32,
	epochNotifier vmcommon.EpochNotifier,
) (*dctGlobalSettings, error) {
	if check.IfNil(accounts) {
		return nil, ErrNilAccountsAdapter
	}
	if !isCorrectFunction(function) {
		return nil, ErrInvalidArguments
	}

	e := &dctGlobalSettings{
		keyPrefix: []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier),
		set:       set,
		accounts:  accounts,
	}

	e.baseEnabled = &baseEnabled{
		function:        function,
		activationEpoch: activationEpoch,
		flagActivated:   atomic.Flag{},
	}

	epochNotifier.RegisterNotifyHandler(e)

	return e, nil
}

func isCorrectFunction(function string) bool {
	switch function {
	case core.BuiltInFunctionDCTPause, core.BuiltInFunctionDCTUnPause, core.BuiltInFunctionDCTSetLimitedTransfer, core.BuiltInFunctionDCTUnSetLimitedTransfer:
		return true
	default:
		return false
	}
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctGlobalSettings) SetNewGasConfig(_ *vmcommon.GasCost) {
}

// ProcessBuiltinFunction resolves DCT pause function call
func (e *dctGlobalSettings) ProcessBuiltinFunction(
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
	if !bytes.Equal(vmInput.CallerAddr, core.DCTSCAddress) {
		return nil, ErrAddressIsNotDCTSystemSC
	}
	if !vmcommon.IsSystemAccountAddress(vmInput.RecipientAddr) {
		return nil, ErrOnlySystemAccountAccepted
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)

	err := e.toggleSetting(dctTokenKey)
	if err != nil {
		return nil, err
	}

	vmOutput := &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}
	return vmOutput, nil
}

func (e *dctGlobalSettings) toggleSetting(token []byte) error {
	systemSCAccount, err := e.getSystemAccount()
	if err != nil {
		return err
	}

	dctMetaData, err := e.getGlobalMetadata(token)
	if err != nil {
		return err
	}

	switch e.function {
	case core.BuiltInFunctionDCTSetLimitedTransfer, core.BuiltInFunctionDCTUnSetLimitedTransfer:
		dctMetaData.LimitedTransfer = e.set
		break
	case core.BuiltInFunctionDCTPause, core.BuiltInFunctionDCTUnPause:
		dctMetaData.Paused = e.set
		break
	}

	err = systemSCAccount.AccountDataHandler().SaveKeyValue(token, dctMetaData.ToBytes())
	if err != nil {
		return err
	}

	return e.accounts.SaveAccount(systemSCAccount)
}

func (e *dctGlobalSettings) getSystemAccount() (vmcommon.UserAccountHandler, error) {
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
func (e *dctGlobalSettings) IsPaused(tokenKey []byte) bool {
	dctMetadata, err := e.getGlobalMetadata(tokenKey)
	if err != nil {
		return false
	}

	return dctMetadata.Paused
}

// IsLimitedTransfer returns true if the token is with limited transfer
func (e *dctGlobalSettings) IsLimitedTransfer(tokenKey []byte) bool {
	dctMetadata, err := e.getGlobalMetadata(tokenKey)
	if err != nil {
		return false
	}

	return dctMetadata.LimitedTransfer
}

func (e *dctGlobalSettings) getGlobalMetadata(tokenKey []byte) (*DCTGlobalMetadata, error) {
	systemSCAccount, err := e.getSystemAccount()
	if err != nil {
		return nil, err
	}

	val, _ := systemSCAccount.AccountDataHandler().RetrieveValue(tokenKey)
	dctMetaData := DCTGlobalMetadataFromBytes(val)
	return &dctMetaData, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctGlobalSettings) IsInterfaceNil() bool {
	return e == nil
}
