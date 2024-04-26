package builtInFunctions

import (
	"bytes"
	"math/big"
	"sync"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/atomic"
)

type dctBurn struct {
	*baseDisabled
	funcGasCost           uint64
	marshalizer           vmcommon.Marshalizer
	keyPrefix             []byte
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler
	mutExecution          sync.RWMutex
}

// NewDCTBurnFunc returns the dct burn built-in function component
func NewDCTBurnFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	disableEpoch uint32,
	epochNotifier vmcommon.EpochNotifier,
) (*dctBurn, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(globalSettingsHandler) {
		return nil, ErrNilGlobalSettingsHandler
	}

	e := &dctBurn{
		funcGasCost:           funcGasCost,
		marshalizer:           marshalizer,
		keyPrefix:             []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier),
		globalSettingsHandler: globalSettingsHandler,
	}

	e.baseDisabled = &baseDisabled{
		function:          core.BuiltInFunctionDCTBurn,
		deActivationEpoch: disableEpoch,
		flagActivated:     atomic.Flag{},
	}

	epochNotifier.RegisterNotifyHandler(e)

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctBurn) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTBurn
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT burn function call
func (e *dctBurn) ProcessBuiltinFunction(
	acntSnd, _ vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	err := checkBasicDCTArguments(vmInput)
	if err != nil {
		return nil, err
	}
	if len(vmInput.Arguments) != 2 {
		return nil, ErrInvalidArguments
	}
	value := big.NewInt(0).SetBytes(vmInput.Arguments[1])
	if value.Cmp(zero) <= 0 {
		return nil, ErrNegativeValue
	}
	if !bytes.Equal(vmInput.RecipientAddr, core.DCTSCAddress) {
		return nil, ErrAddressIsNotDCTSystemSC
	}
	if check.IfNil(acntSnd) {
		return nil, ErrNilUserAccount
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)

	if vmInput.GasProvided < e.funcGasCost {
		return nil, ErrNotEnoughGas
	}

	err = addToDCTBalance(acntSnd, dctTokenKey, big.NewInt(0).Neg(value), e.marshalizer, e.globalSettingsHandler, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	gasRemaining := computeGasRemaining(acntSnd, vmInput.GasProvided, e.funcGasCost)
	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}
	if vmcommon.IsSmartContractAddress(vmInput.CallerAddr) {
		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			core.BuiltInFunctionDCTBurn,
			vmInput.Arguments,
			vmInput.RecipientAddr,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTBurn), vmInput.Arguments[0], 0, value, vmInput.CallerAddr)

	return vmOutput, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctBurn) IsInterfaceNil() bool {
	return e == nil
}
