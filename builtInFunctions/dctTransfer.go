package builtInFunctions

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"sync"

	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/check"
	"github.com/numbatx/gn-vm-common/data/dct"
)

var zero = big.NewInt(0)

type dctTransfer struct {
	baseAlwaysActive
	funcGasCost      uint64
	marshalizer      vmcommon.Marshalizer
	keyPrefix        []byte
	pauseHandler     vmcommon.DCTPauseHandler
	payableHandler   vmcommon.PayableHandler
	shardCoordinator vmcommon.Coordinator
	mutExecution     sync.RWMutex
}

// NewDCTTransferFunc returns the dct transfer built-in function component
func NewDCTTransferFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	pauseHandler vmcommon.DCTPauseHandler,
	shardCoordinator vmcommon.Coordinator,
) (*dctTransfer, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(pauseHandler) {
		return nil, ErrNilPauseHandler
	}
	if check.IfNil(shardCoordinator) {
		return nil, ErrNilShardCoordinator
	}

	e := &dctTransfer{
		funcGasCost:      funcGasCost,
		marshalizer:      marshalizer,
		keyPrefix:        []byte(vmcommon.NumbatProtectedKeyPrefix + vmcommon.DCTKeyIdentifier),
		pauseHandler:     pauseHandler,
		payableHandler:   &disabledPayableHandler{},
		shardCoordinator: shardCoordinator,
	}

	return e, nil
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
	if e.shardCoordinator.ComputeId(vmInput.RecipientAddr) == vmcommon.MetachainShardId {
		return nil, ErrInvalidRcvAddr
	}

	value := big.NewInt(0).SetBytes(vmInput.Arguments[1])
	if value.Cmp(zero) <= 0 {
		return nil, ErrNegativeValue
	}

	gasRemaining := computeGasRemaining(acntSnd, vmInput.GasProvided, e.funcGasCost)
	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)
	tokenID := vmInput.Arguments[0]

	if !check.IfNil(acntSnd) {
		// gas is paid only by sender
		if vmInput.GasProvided < e.funcGasCost {
			return nil, ErrNotEnoughGas
		}

		err = addToDCTBalance(acntSnd, dctTokenKey, big.NewInt(0).Neg(value), e.marshalizer, e.pauseHandler, vmInput.ReturnCallAfterError)
		if err != nil {
			return nil, err
		}
	}

	isSCCallAfter := vmcommon.IsSmartContractAddress(vmInput.RecipientAddr) && len(vmInput.Arguments) > vmcommon.MinLenArgumentsDCTTransfer

	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}
	if !check.IfNil(acntDst) {
		if mustVerifyPayable(vmInput, vmcommon.MinLenArgumentsDCTTransfer) {
			isPayable, errPayable := e.payableHandler.IsPayable(vmInput.RecipientAddr)
			if errPayable != nil {
				return nil, errPayable
			}
			if !isPayable {
				return nil, ErrAccountNotPayable
			}
		}

		err = addToDCTBalance(acntDst, dctTokenKey, value, e.marshalizer, e.pauseHandler, vmInput.ReturnCallAfterError)
		if err != nil {
			return nil, err
		}

		if isSCCallAfter {
			vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
			var callArgs [][]byte
			if len(vmInput.Arguments) > vmcommon.MinLenArgumentsDCTTransfer+1 {
				callArgs = vmInput.Arguments[vmcommon.MinLenArgumentsDCTTransfer+1:]
			}

			addOutputTransferToVMOutput(
				vmInput.CallerAddr,
				string(vmInput.Arguments[vmcommon.MinLenArgumentsDCTTransfer]),
				callArgs,
				vmInput.RecipientAddr,
				vmInput.GasLocked,
				vmInput.CallType,
				vmOutput)

			addDCTEntryInVMOutput(vmOutput, []byte(vmcommon.BuiltInFunctionDCTTransfer), tokenID, value, vmInput.CallerAddr, acntDst.AddressBytes())
			return vmOutput, nil
		}

		if vmInput.CallType == vmcommon.AsynchronousCallBack && check.IfNil(acntSnd) {
			// gas was already consumed on sender shard
			vmOutput.GasRemaining = vmInput.GasProvided
		}

		addDCTEntryInVMOutput(vmOutput, []byte(vmcommon.BuiltInFunctionDCTTransfer), tokenID, value, vmInput.CallerAddr, acntDst.AddressBytes())
		return vmOutput, nil
	}

	// cross-shard DCT transfer call through a smart contract
	if vmcommon.IsSmartContractAddress(vmInput.CallerAddr) {
		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			vmcommon.BuiltInFunctionDCTTransfer,
			vmInput.Arguments,
			vmInput.RecipientAddr,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	addDCTEntryInVMOutput(vmOutput, []byte(vmcommon.BuiltInFunctionDCTTransfer), tokenID, value, vmInput.CallerAddr)
	return vmOutput, nil
}

func mustVerifyPayable(vmInput *vmcommon.ContractCallInput, minLenArguments int) bool {
	if vmInput.CallType == vmcommon.AsynchronousCallBack || vmInput.CallType == vmcommon.DCTTransferAndExecute {
		return false
	}
	if bytes.Equal(vmInput.CallerAddr, vmcommon.DCTSCAddress) {
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
	callType vmcommon.CallType,
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
	pauseHandler vmcommon.DCTPauseHandler,
	isReturnWithError bool,
) error {
	dctData, err := getDCTDataFromKey(userAcnt, key, marshalizer)
	if err != nil {
		return err
	}

	if dctData.Type != uint32(vmcommon.Fungible) {
		return ErrOnlyFungibleTokensHaveBalanceTransfer
	}

	err = checkFrozeAndPause(userAcnt.AddressBytes(), key, dctData, pauseHandler, isReturnWithError)
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
	pauseHandler vmcommon.DCTPauseHandler,
	isReturnWithError bool,
) error {
	if isReturnWithError {
		return nil
	}
	if bytes.Equal(senderAddr, vmcommon.DCTSCAddress) {
		return nil
	}

	dctUserMetaData := DCTUserMetadataFromBytes(dctData.Properties)
	if dctUserMetaData.Frozen {
		return ErrDCTIsFrozenForAccount
	}

	if pauseHandler.IsPaused(key) {
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
	dctData := &dct.DCToken{Value: big.NewInt(0), Type: uint32(vmcommon.Fungible)}
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
