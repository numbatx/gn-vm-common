package builtInFunctions

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/check"
)

type dctLocalMint struct {
	baseAlwaysActive
	keyPrefix    []byte
	marshalizer  vmcommon.Marshalizer
	pauseHandler vmcommon.DCTPauseHandler
	rolesHandler vmcommon.DCTRoleHandler
	funcGasCost  uint64
	mutExecution sync.RWMutex
}

// NewDCTLocalMintFunc returns the dct local mint built-in function component
func NewDCTLocalMintFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	pauseHandler vmcommon.DCTPauseHandler,
	rolesHandler vmcommon.DCTRoleHandler,
) (*dctLocalMint, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(pauseHandler) {
		return nil, ErrNilPauseHandler
	}
	if check.IfNil(rolesHandler) {
		return nil, ErrNilRolesHandler
	}

	e := &dctLocalMint{
		keyPrefix:    []byte(vmcommon.NumbatProtectedKeyPrefix + vmcommon.DCTKeyIdentifier),
		marshalizer:  marshalizer,
		pauseHandler: pauseHandler,
		rolesHandler: rolesHandler,
		funcGasCost:  funcGasCost,
		mutExecution: sync.RWMutex{},
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctLocalMint) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTLocalMint
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT local mint function call
func (e *dctLocalMint) ProcessBuiltinFunction(
	acntSnd, _ vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	err := checkInputArgumentsForLocalAction(acntSnd, vmInput, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	tokenID := vmInput.Arguments[0]
	err = e.rolesHandler.CheckAllowedToExecute(acntSnd, tokenID, []byte(vmcommon.DCTRoleLocalMint))
	if err != nil {
		return nil, err
	}

	if len(vmInput.Arguments[1]) > vmcommon.MaxLenForDCTIssueMint {
		return nil, fmt.Errorf("%w max length for dct issue is %d", ErrInvalidArguments, vmcommon.MaxLenForDCTIssueMint)
	}

	value := big.NewInt(0).SetBytes(vmInput.Arguments[1])
	dctTokenKey := append(e.keyPrefix, tokenID...)
	err = addToDCTBalance(acntSnd, dctTokenKey, big.NewInt(0).Set(value), e.marshalizer, e.pauseHandler, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	vmOutput := &vmcommon.VMOutput{ReturnCode: vmcommon.Ok, GasRemaining: vmInput.GasProvided - e.funcGasCost}

	addDCTEntryInVMOutput(vmOutput, []byte(vmcommon.BuiltInFunctionDCTLocalMint), vmInput.Arguments[0], value, vmInput.CallerAddr)

	return vmOutput, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctLocalMint) IsInterfaceNil() bool {
	return e == nil
}
