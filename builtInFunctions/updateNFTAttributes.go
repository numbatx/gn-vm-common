package builtInFunctions

import (
	"math/big"
	"sync"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-vm-common"
)

type dctNFTupdate struct {
	baseActiveHandler
	keyPrefix             []byte
	dctStorageHandler    vmcommon.DCTNFTStorageHandler
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler
	rolesHandler          vmcommon.DCTRoleHandler
	gasConfig             vmcommon.BaseOperationCost
	funcGasCost           uint64
	mutExecution          sync.RWMutex
}

// NewDCTNFTUpdateAttributesFunc returns the dct NFT update attribute built-in function component
func NewDCTNFTUpdateAttributesFunc(
	funcGasCost uint64,
	gasConfig vmcommon.BaseOperationCost,
	dctStorageHandler vmcommon.DCTNFTStorageHandler,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	rolesHandler vmcommon.DCTRoleHandler,
	enableEpochsHandler vmcommon.EnableEpochsHandler,
) (*dctNFTupdate, error) {
	if check.IfNil(dctStorageHandler) {
		return nil, ErrNilDCTNFTStorageHandler
	}
	if check.IfNil(globalSettingsHandler) {
		return nil, ErrNilGlobalSettingsHandler
	}
	if check.IfNil(rolesHandler) {
		return nil, ErrNilRolesHandler
	}
	if check.IfNil(enableEpochsHandler) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &dctNFTupdate{
		keyPrefix:             []byte(baseDCTKeyPrefix),
		dctStorageHandler:    dctStorageHandler,
		funcGasCost:           funcGasCost,
		mutExecution:          sync.RWMutex{},
		globalSettingsHandler: globalSettingsHandler,
		gasConfig:             gasConfig,
		rolesHandler:          rolesHandler,
	}

	e.baseActiveHandler.activeHandler = enableEpochsHandler.IsDCTNFTImprovementV1FlagEnabled

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctNFTupdate) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTNFTUpdateAttributes
	e.gasConfig = gasCost.BaseOperationCost
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT NFT update attributes function call
// Requires 3 arguments:
// arg0 - token identifier
// arg1 - nonce
// arg2 - new attributes
func (e *dctNFTupdate) ProcessBuiltinFunction(
	acntSnd, _ vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	err := checkDCTNFTCreateBurnAddInput(acntSnd, vmInput, e.funcGasCost)
	if err != nil {
		return nil, err
	}
	if len(vmInput.Arguments) != 3 {
		return nil, ErrInvalidArguments
	}

	err = e.rolesHandler.CheckAllowedToExecute(acntSnd, vmInput.Arguments[0], []byte(core.DCTRoleNFTUpdateAttributes))
	if err != nil {
		return nil, err
	}

	gasCostForStore := uint64(len(vmInput.Arguments[2])) * e.gasConfig.StorePerByte
	if vmInput.GasProvided < e.funcGasCost+gasCostForStore {
		return nil, ErrNotEnoughGas
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)
	nonce := big.NewInt(0).SetBytes(vmInput.Arguments[1]).Uint64()
	if nonce == 0 {
		return nil, ErrNFTDoesNotHaveMetadata
	}
	dctData, err := e.dctStorageHandler.GetDCTNFTTokenOnSender(acntSnd, dctTokenKey, nonce)
	if err != nil {
		return nil, err
	}

	dctData.TokenMetaData.Attributes = vmInput.Arguments[2]

	_, err = e.dctStorageHandler.SaveDCTNFTToken(acntSnd.AddressBytes(), acntSnd, dctTokenKey, nonce, dctData, true, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	vmOutput := &vmcommon.VMOutput{
		ReturnCode:   vmcommon.Ok,
		GasRemaining: vmInput.GasProvided - e.funcGasCost - gasCostForStore,
	}

	addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTNFTUpdateAttributes), vmInput.Arguments[0], nonce, big.NewInt(0), vmInput.CallerAddr, vmInput.Arguments[2])

	return vmOutput, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctNFTupdate) IsInterfaceNil() bool {
	return e == nil
}
