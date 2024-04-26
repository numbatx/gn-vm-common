package builtInFunctions

import (
	"math/big"
	"sync"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-vm-common"
)

type dctNFTBurn struct {
	baseAlwaysActive
	keyPrefix             []byte
	marshalizer           vmcommon.Marshalizer
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler
	rolesHandler          vmcommon.DCTRoleHandler
	funcGasCost           uint64
	mutExecution          sync.RWMutex
}

// NewDCTNFTBurnFunc returns the dct NFT burn built-in function component
func NewDCTNFTBurnFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	rolesHandler vmcommon.DCTRoleHandler,
) (*dctNFTBurn, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(globalSettingsHandler) {
		return nil, ErrNilGlobalSettingsHandler
	}
	if check.IfNil(rolesHandler) {
		return nil, ErrNilRolesHandler
	}

	e := &dctNFTBurn{
		keyPrefix:             []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier),
		marshalizer:           marshalizer,
		globalSettingsHandler: globalSettingsHandler,
		rolesHandler:          rolesHandler,
		funcGasCost:           funcGasCost,
		mutExecution:          sync.RWMutex{},
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctNFTBurn) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTNFTBurn
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT NFT burn function call
// Requires 3 arguments:
// arg0 - token identifier
// arg1 - nonce
// arg2 - quantity to burn
func (e *dctNFTBurn) ProcessBuiltinFunction(
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

	err = e.rolesHandler.CheckAllowedToExecute(acntSnd, vmInput.Arguments[0], []byte(core.DCTRoleNFTBurn))
	if err != nil {
		return nil, err
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)
	nonce := big.NewInt(0).SetBytes(vmInput.Arguments[1]).Uint64()
	dctData, err := getDCTNFTTokenOnSender(acntSnd, dctTokenKey, nonce, e.marshalizer)
	if err != nil {
		return nil, err
	}
	if nonce == 0 {
		return nil, ErrNFTDoesNotHaveMetadata
	}

	quantityToBurn := big.NewInt(0).SetBytes(vmInput.Arguments[2])
	if dctData.Value.Cmp(quantityToBurn) < 0 {
		return nil, ErrInvalidNFTQuantity
	}

	dctData.Value.Sub(dctData.Value, quantityToBurn)

	_, err = saveDCTNFTToken(acntSnd, dctTokenKey, dctData, e.marshalizer, e.globalSettingsHandler, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	vmOutput := &vmcommon.VMOutput{
		ReturnCode:   vmcommon.Ok,
		GasRemaining: vmInput.GasProvided - e.funcGasCost,
	}

	addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTNFTBurn), vmInput.Arguments[0], nonce, quantityToBurn, vmInput.CallerAddr)

	return vmOutput, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctNFTBurn) IsInterfaceNil() bool {
	return e == nil
}
