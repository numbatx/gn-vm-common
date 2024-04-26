package builtInFunctions

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"sync"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-core/data/vm"
	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/atomic"
)

var zero = big.NewInt(0)

type dctTransfer struct {
	baseAlwaysActive
	funcGasCost           uint64
	marshalizer           vmcommon.Marshalizer
	keyPrefix             []byte
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler
	payableHandler        vmcommon.PayableHandler
	shardCoordinator      vmcommon.Coordinator
	mutExecution          sync.RWMutex

	rolesHandler              vmcommon.DCTRoleHandler
	transferToMetaEnableEpoch uint32
	flagTransferToMeta        atomic.Flag
}

// NewDCTTransferFunc returns the dct transfer built-in function component
func NewDCTTransferFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	shardCoordinator vmcommon.Coordinator,
	rolesHandler vmcommon.DCTRoleHandler,
	transferToMetaEnableEpoch uint32,
	epochNotifier vmcommon.EpochNotifier,
) (*dctTransfer, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(globalSettingsHandler) {
		return nil, ErrNilGlobalSettingsHandler
	}
	if check.IfNil(shardCoordinator) {
		return nil, ErrNilShardCoordinator
	}
	if check.IfNil(rolesHandler) {
		return nil, ErrNilRolesHandler
	}
	if check.IfNil(epochNotifier) {
		return nil, ErrNilEpochHandler
	}

	e := &dctTransfer{
		funcGasCost:               funcGasCost,
		marshalizer:               marshalizer,
		keyPrefix:                 []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier),
		globalSettingsHandler:     globalSettingsHandler,
		payableHandler:            &disabledPayableHandler{},
		shardCoordinator:          shardCoordinator,
		rolesHandler:              rolesHandler,
		transferToMetaEnableEpoch: transferToMetaEnableEpoch,
	}

	epochNotifier.RegisterNotifyHandler(e)

	return e, nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (e *dctTransfer) EpochConfirmed(epoch uint32, _ uint64) {
	e.flagTransferToMeta.Toggle(epoch >= e.transferToMetaEnableEpoch)
	log.Debug("DCT transfer to metachain flag", "enabled", e.flagTransferToMeta.IsSet())
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctTransfer) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTTransfer
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT transfer function calls
func (e *dctTransfer) ProcessBuiltinFunction(
	acntSnd, acntDst vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	err := checkBasicDCTArguments(vmInput)
	if err != nil {
		return nil, err
	}
	isInvalidTransferToMeta := e.shardCoordinator.ComputeId(vmInput.RecipientAddr) == core.MetachainShardId && !e.flagTransferToMeta.IsSet()
	if isInvalidTransferToMeta {
		return nil, ErrInvalidRcvAddr
	}

	value := big.NewInt(0).SetBytes(vmInput.Arguments[1])
	if value.Cmp(zero) <= 0 {
		return nil, ErrNegativeValue
	}

	gasRemaining := computeGasRemaining(acntSnd, vmInput.GasProvided, e.funcGasCost)
	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)
	tokenID := vmInput.Arguments[0]

	err = checkIfTransferCanHappenWithLimitedTransfer(dctTokenKey, e.globalSettingsHandler, e.rolesHandler, acntSnd, acntDst, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	if !check.IfNil(acntSnd) {
		// gas is paid only by sender
		if vmInput.GasProvided < e.funcGasCost {
			return nil, ErrNotEnoughGas
		}

		err = addToDCTBalance(acntSnd, dctTokenKey, big.NewInt(0).Neg(value), e.marshalizer, e.globalSettingsHandler, vmInput.ReturnCallAfterError)
		if err != nil {
			return nil, err
		}
	}

	isSCCallAfter := determineIsSCCallAfter(vmInput, vmInput.RecipientAddr, core.MinLenArgumentsDCTTransfer)
	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}
	if !check.IfNil(acntDst) {
		if mustVerifyPayable(vmInput, core.MinLenArgumentsDCTTransfer) {
			isPayable, errPayable := e.payableHandler.IsPayable(vmInput.CallerAddr, vmInput.RecipientAddr)
			if errPayable != nil {
				return nil, errPayable
			}
			if !isPayable {
				return nil, ErrAccountNotPayable
			}
		}

		err = addToDCTBalance(acntDst, dctTokenKey, value, e.marshalizer, e.globalSettingsHandler, vmInput.ReturnCallAfterError)
		if err != nil {
			return nil, err
		}

		if isSCCallAfter {
			vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
			var callArgs [][]byte
			if len(vmInput.Arguments) > core.MinLenArgumentsDCTTransfer+1 {
				callArgs = vmInput.Arguments[core.MinLenArgumentsDCTTransfer+1:]
			}

			addOutputTransferToVMOutput(
				vmInput.CallerAddr,
				string(vmInput.Arguments[core.MinLenArgumentsDCTTransfer]),
				callArgs,
				vmInput.RecipientAddr,
				vmInput.GasLocked,
				vmInput.CallType,
				vmOutput)

			addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTTransfer), tokenID, 0, value, vmInput.CallerAddr, acntDst.AddressBytes())
			return vmOutput, nil
		}

		if vmInput.CallType == vm.AsynchronousCallBack && check.IfNil(acntSnd) {
			// gas was already consumed on sender shard
			vmOutput.GasRemaining = vmInput.GasProvided
		}

		addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTTransfer), tokenID, 0, value, vmInput.CallerAddr, acntDst.AddressBytes())
		return vmOutput, nil
	}

	// cross-shard DCT transfer call through a smart contract
	if vmcommon.IsSmartContractAddress(vmInput.CallerAddr) {
		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			core.BuiltInFunctionDCTTransfer,
			vmInput.Arguments,
			vmInput.RecipientAddr,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTTransfer), tokenID, 0, value, vmInput.CallerAddr, vmInput.RecipientAddr)
	return vmOutput, nil
}

func determineIsSCCallAfter(vmInput *vmcommon.ContractCallInput, destAddress []byte, minLenArguments int) bool {
	if len(vmInput.Arguments) <= minLenArguments {
		return false
	}
	if vmInput.ReturnCallAfterError && vmInput.CallType != vm.AsynchronousCallBack {
		return false
	}
	if !vmcommon.IsSmartContractAddress(destAddress) {
		return false
	}

	return true
}

func mustVerifyPayable(vmInput *vmcommon.ContractCallInput, minLenArguments int) bool {
	if vmInput.CallType == vm.AsynchronousCall || vmInput.CallType == vm.DCTTransferAndExecute {
		return false
	}
	if bytes.Equal(vmInput.CallerAddr, core.DCTSCAddress) {
		return false
	}

	if len(vmInput.Arguments) > minLenArguments {
		return false
	}

	return true
}

func addOutputTransferToVMOutput(
	senderAddress []byte,
	function string,
	arguments [][]byte,
	recipient []byte,
	gasLocked uint64,
	callType vm.CallType,
	vmOutput *vmcommon.VMOutput,
) {
	dctTransferTxData := function
	for _, arg := range arguments {
		dctTransferTxData += "@" + hex.EncodeToString(arg)
	}
	outTransfer := vmcommon.OutputTransfer{
		Value:         big.NewInt(0),
		GasLimit:      vmOutput.GasRemaining,
		GasLocked:     gasLocked,
		Data:          []byte(dctTransferTxData),
		CallType:      callType,
		SenderAddress: senderAddress,
	}
	vmOutput.OutputAccounts = make(map[string]*vmcommon.OutputAccount)
	vmOutput.OutputAccounts[string(recipient)] = &vmcommon.OutputAccount{
		Address:         recipient,
		OutputTransfers: []vmcommon.OutputTransfer{outTransfer},
	}
	vmOutput.GasRemaining = 0
}

func addToDCTBalance(
	userAcnt vmcommon.UserAccountHandler,
	key []byte,
	value *big.Int,
	marshalizer vmcommon.Marshalizer,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	isReturnWithError bool,
) error {
	dctData, err := getDCTDataFromKey(userAcnt, key, marshalizer)
	if err != nil {
		return err
	}

	if dctData.Type != uint32(core.Fungible) {
		return ErrOnlyFungibleTokensHaveBalanceTransfer
	}

	err = checkFrozeAndPause(userAcnt.AddressBytes(), key, dctData, globalSettingsHandler, isReturnWithError)
	if err != nil {
		return err
	}

	dctData.Value.Add(dctData.Value, value)
	if dctData.Value.Cmp(zero) < 0 {
		return ErrInsufficientFunds
	}

	err = saveDCTData(userAcnt, dctData, key, marshalizer)
	if err != nil {
		return err
	}

	return nil
}

func checkFrozeAndPause(
	senderAddr []byte,
	key []byte,
	dctData *dct.DCToken,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	isReturnWithError bool,
) error {
	if isReturnWithError {
		return nil
	}
	if bytes.Equal(senderAddr, core.DCTSCAddress) {
		return nil
	}

	dctUserMetaData := DCTUserMetadataFromBytes(dctData.Properties)
	if dctUserMetaData.Frozen {
		return ErrDCTIsFrozenForAccount
	}

	if globalSettingsHandler.IsPaused(key) {
		return ErrDCTTokenIsPaused
	}

	return nil
}

func arePropertiesEmpty(properties []byte) bool {
	for _, property := range properties {
		if property != 0 {
			return false
		}
	}
	return true
}

func saveDCTData(
	userAcnt vmcommon.UserAccountHandler,
	dctData *dct.DCToken,
	key []byte,
	marshalizer vmcommon.Marshalizer,
) error {
	isValueZero := dctData.Value.Cmp(zero) == 0
	if isValueZero && arePropertiesEmpty(dctData.Properties) {
		return userAcnt.AccountDataHandler().SaveKeyValue(key, nil)
	}

	marshaledData, err := marshalizer.Marshal(dctData)
	if err != nil {
		return err
	}

	return userAcnt.AccountDataHandler().SaveKeyValue(key, marshaledData)
}

func getDCTDataFromKey(
	userAcnt vmcommon.UserAccountHandler,
	key []byte,
	marshalizer vmcommon.Marshalizer,
) (*dct.DCToken, error) {
	dctData := &dct.DCToken{Value: big.NewInt(0), Type: uint32(core.Fungible)}
	marshaledData, err := userAcnt.AccountDataHandler().RetrieveValue(key)
	if err != nil || len(marshaledData) == 0 {
		return dctData, nil
	}

	err = marshalizer.Unmarshal(dctData, marshaledData)
	if err != nil {
		return nil, err
	}

	return dctData, nil
}

// will return nil if transfer is not limited
// if we are at sender shard, the sender or the destination must have the transfer role
// we cannot transfer a limited dct to destination shard, as there we do not know if that token was transferred or not
// by an account with transfer account
func checkIfTransferCanHappenWithLimitedTransfer(
	tokenID []byte,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	roleHandler vmcommon.DCTRoleHandler,
	acntSnd, acntDst vmcommon.UserAccountHandler,
	isReturnWithError bool,
) error {
	if isReturnWithError {
		return nil
	}
	if check.IfNil(acntSnd) {
		return nil
	}
	if !globalSettingsHandler.IsLimitedTransfer(tokenID) {
		return nil
	}

	errSender := roleHandler.CheckAllowedToExecute(acntSnd, tokenID, []byte(core.DCTRoleTransfer))
	if errSender == nil {
		return nil
	}

	errDestination := roleHandler.CheckAllowedToExecute(acntDst, tokenID, []byte(core.DCTRoleTransfer))
	return errDestination
}

// SetPayableHandler will set the payable handler to the function
func (e *dctTransfer) SetPayableHandler(payableHandler vmcommon.PayableHandler) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctTransfer) IsInterfaceNil() bool {
	return e == nil
}
