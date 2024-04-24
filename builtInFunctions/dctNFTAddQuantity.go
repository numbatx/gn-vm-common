package builtInFunctions

import (
	"math/big"
	"sync"

	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/check"
)

type dctNFTAddQuantity struct {
	baseAlwaysActive
	keyPrefix    []byte
	marshalizer  vmcommon.Marshalizer
	pauseHandler vmcommon.DCTPauseHandler
	rolesHandler vmcommon.DCTRoleHandler
	funcGasCost  uint64
	mutExecution sync.RWMutex
}

// NewDCTNFTAddQuantityFunc returns the dct NFT add quantity built-in function component
func NewDCTNFTAddQuantityFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	pauseHandler vmcommon.DCTPauseHandler,
	rolesHandler vmcommon.DCTRoleHandler,
) (*dctNFTAddQuantity, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(pauseHandler) {
		return nil, ErrNilPauseHandler
	}
	if check.IfNil(rolesHandler) {
		return nil, ErrNilRolesHandler
	}

	e := &dctNFTAddQuantity{
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
func (e *dctNFTAddQuantity) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTNFTAddQuantity
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT NFT add quantity function call
// Requires 3 arguments:
// arg0 - token identifier
// arg1 - nonce
// arg2 - quantity to add
func (e *dctNFTAddQuantity) ProcessBuiltinFunction(
	acntSnd, _ vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	err := checkDCTNFTCreateBurnAddInput(acntSnd, vmInput, e.funcGasCost)
	if err != nil {
		return nil, err
	}
	if len(vmInput.Arguments) < 3 {
		return nil, ErrInvalidArguments
	}

	err = e.rolesHandler.CheckAllowedToExecute(acntSnd, vmInput.Arguments[0], []byte(vmcommon.DCTRoleNFTAddQuantity))
	if err != nil {
		return nil, err
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)
	nonce := big.NewInt(0).SetBytes(vmInput.Arguments[1]).Uint64()
	if nonce == 0 {
		return nil, ErrNFTDoesNotHaveMetadata
	}
	dctData, err := getDCTNFTTokenOnSender(acntSnd, dctTokenKey, nonce, e.marshalizer)
	if err != nil {
		return nil, err
	}

	dctData.Value.Add(dctData.Value, big.NewInt(0).SetBytes(vmInput.Arguments[2]))

	_, err = saveDCTNFTToken(acntSnd, dctTokenKey, dctData, e.marshalizer, e.pauseHandler, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	logEntry := newEntryForNFT(vmcommon.BuiltInFunctionDCTNFTAddQuantity, vmInput.CallerAddr, vmInput.Arguments[0], nonce)
	vmOutput := &vmcommon.VMOutput{
		ReturnCode:   vmcommon.Ok,
		GasRemaining: vmInput.GasProvided - e.funcGasCost,
		Logs:         []*vmcommon.LogEntry{logEntry},
	}
	return vmOutput, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctNFTAddQuantity) IsInterfaceNil() bool {
	return e == nil
}
