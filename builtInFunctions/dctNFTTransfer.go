package builtInFunctions

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/check"
	"github.com/numbatx/gn-vm-common/data/dct"
)

type dctNFTTransfer struct {
	baseAlwaysActive
	keyPrefix        []byte
	marshalizer      vmcommon.Marshalizer
	pauseHandler     vmcommon.DCTPauseHandler
	payableHandler   vmcommon.PayableHandler
	funcGasCost      uint64
	accounts         vmcommon.AccountsAdapter
	shardCoordinator vmcommon.Coordinator
	gasConfig        vmcommon.BaseOperationCost
	mutExecution     sync.RWMutex
}

// NewDCTNFTTransferFunc returns the dct NFT transfer built-in function component
func NewDCTNFTTransferFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	pauseHandler vmcommon.DCTPauseHandler,
	accounts vmcommon.AccountsAdapter,
	shardCoordinator vmcommon.Coordinator,
	gasConfig vmcommon.BaseOperationCost,
) (*dctNFTTransfer, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(pauseHandler) {
		return nil, ErrNilPauseHandler
	}
	if check.IfNil(accounts) {
		return nil, ErrNilAccountsAdapter
	}
	if check.IfNil(shardCoordinator) {
		return nil, ErrNilShardCoordinator
	}

	e := &dctNFTTransfer{
		keyPrefix:        []byte(vmcommon.NumbatProtectedKeyPrefix + vmcommon.DCTKeyIdentifier),
		marshalizer:      marshalizer,
		pauseHandler:     pauseHandler,
		funcGasCost:      funcGasCost,
		accounts:         accounts,
		shardCoordinator: shardCoordinator,
		gasConfig:        gasConfig,
		mutExecution:     sync.RWMutex{},
		payableHandler:   &disabledPayableHandler{},
	}

	return e, nil
}

// SetPayableHandler will set the payable handler to the function
func (e *dctNFTTransfer) SetPayableHandler(payableHandler vmcommon.PayableHandler) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctNFTTransfer) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTNFTTransfer
	e.gasConfig = gasCost.BaseOperationCost
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT NFT transfer roles function call
// Requires 4 arguments:
// arg0 - token identifier
// arg1 - nonce
// arg2 - quantity to transfer
// arg3 - destination address
// if cross-shard, the rest of arguments will be filled inside the SCR
func (e *dctNFTTransfer) ProcessBuiltinFunction(
	acntSnd, acntDst vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	err := checkBasicDCTArguments(vmInput)
	if err != nil {
		return nil, err
	}
	if len(vmInput.Arguments) < 4 {
		return nil, ErrInvalidArguments
	}

	if bytes.Equal(vmInput.CallerAddr, vmInput.RecipientAddr) {
		return e.processNFTTransferOnSenderShard(acntSnd, vmInput)
	}

	// in cross shard NFT transfer the sender account must be nil
	if !check.IfNil(acntSnd) {
		return nil, ErrInvalidRcvAddr
	}
	if check.IfNil(acntDst) {
		return nil, ErrInvalidRcvAddr
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)
	marshaledNFTTransfer := vmInput.Arguments[3]
	dctTransferData := &dct.DCToken{}
	err = e.marshalizer.Unmarshal(dctTransferData, marshaledNFTTransfer)
	if err != nil {
		return nil, err
	}

	err = e.addNFTToDestination(vmInput.RecipientAddr, acntDst, dctTransferData, dctTokenKey, mustVerifyPayable(vmInput, vmcommon.MinLenArgumentsDCTNFTTransfer), vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	// no need to consume gas on destination - sender already paid for it
	vmOutput := &vmcommon.VMOutput{GasRemaining: vmInput.GasProvided}
	if len(vmInput.Arguments) > vmcommon.MinLenArgumentsDCTNFTTransfer && vmcommon.IsSmartContractAddress(vmInput.RecipientAddr) {
		var callArgs [][]byte
		if len(vmInput.Arguments) > vmcommon.MinLenArgumentsDCTNFTTransfer+1 {
			callArgs = vmInput.Arguments[vmcommon.MinLenArgumentsDCTNFTTransfer+1:]
		}

		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			string(vmInput.Arguments[vmcommon.MinLenArgumentsDCTNFTTransfer]),
			callArgs,
			vmInput.RecipientAddr,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	tokenNonce := dctTransferData.TokenMetaData.Nonce
	logEntry := newEntryForNFT(vmcommon.BuiltInFunctionDCTNFTTransfer, vmInput.CallerAddr, vmInput.Arguments[0], tokenNonce)
	logEntry.Topics = append(logEntry.Topics, acntDst.AddressBytes())
	vmOutput.Logs = []*vmcommon.LogEntry{logEntry}

	return vmOutput, nil
}

func (e *dctNFTTransfer) processNFTTransferOnSenderShard(
	acntSnd vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	dstAddress := vmInput.Arguments[3]
	if len(dstAddress) != len(vmInput.CallerAddr) {
		return nil, fmt.Errorf("%w, not a valid destination address", ErrInvalidArguments)
	}
	if bytes.Equal(dstAddress, vmInput.CallerAddr) {
		return nil, fmt.Errorf("%w, can not transfer to self", ErrInvalidArguments)
	}
	if e.shardCoordinator.ComputeId(dstAddress) == vmcommon.MetachainShardId {
		return nil, ErrInvalidRcvAddr
	}
	if vmInput.GasProvided < e.funcGasCost {
		return nil, ErrNotEnoughGas
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

	quantityToTransfer := big.NewInt(0).SetBytes(vmInput.Arguments[2])
	if dctData.Value.Cmp(quantityToTransfer) < 0 {
		return nil, ErrInvalidNFTQuantity
	}
	dctData.Value.Sub(dctData.Value, quantityToTransfer)

	_, err = saveDCTNFTToken(acntSnd, dctTokenKey, dctData, e.marshalizer, e.pauseHandler, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	dctData.Value.Set(quantityToTransfer)

	if e.shardCoordinator.SelfId() == e.shardCoordinator.ComputeId(dstAddress) {
		accountHandler, errLoad := e.accounts.LoadAccount(dstAddress)
		if errLoad != nil {
			return nil, errLoad
		}
		userAccount, ok := accountHandler.(vmcommon.UserAccountHandler)
		if !ok {
			return nil, ErrWrongTypeAssertion
		}

		err = e.addNFTToDestination(dstAddress, userAccount, dctData, dctTokenKey, mustVerifyPayable(vmInput, vmcommon.MinLenArgumentsDCTNFTTransfer), vmInput.ReturnCallAfterError)
		if err != nil {
			return nil, err
		}

		err = e.accounts.SaveAccount(userAccount)
		if err != nil {
			return nil, err
		}
	}

	vmOutput := &vmcommon.VMOutput{
		ReturnCode:   vmcommon.Ok,
		GasRemaining: vmInput.GasProvided - e.funcGasCost,
	}
	err = e.createNFTOutputTransfers(vmInput, vmOutput, dctData, dstAddress)
	if err != nil {
		return nil, err
	}

	tokenNonce := dctData.TokenMetaData.Nonce
	logEntry := newEntryForNFT(vmcommon.BuiltInFunctionDCTNFTTransfer, vmInput.CallerAddr, vmInput.Arguments[0], tokenNonce)
	logEntry.Topics = append(logEntry.Topics, dstAddress)
	vmOutput.Logs = []*vmcommon.LogEntry{logEntry}

	return vmOutput, nil
}

func (e *dctNFTTransfer) createNFTOutputTransfers(
	vmInput *vmcommon.ContractCallInput,
	vmOutput *vmcommon.VMOutput,
	dctTransferData *dct.DCToken,
	dstAddress []byte,
) error {
	marshaledNFTTransfer, err := e.marshalizer.Marshal(dctTransferData)
	if err != nil {
		return err
	}

	gasForTransfer := uint64(len(marshaledNFTTransfer)) * e.gasConfig.DataCopyPerByte
	if gasForTransfer > vmOutput.GasRemaining {
		return ErrNotEnoughGas
	}
	vmOutput.GasRemaining -= gasForTransfer

	nftTransferCallArgs := make([][]byte, 0)
	nftTransferCallArgs = append(nftTransferCallArgs, vmInput.Arguments[:3]...)
	nftTransferCallArgs = append(nftTransferCallArgs, marshaledNFTTransfer)
	if len(vmInput.Arguments) > vmcommon.MinLenArgumentsDCTNFTTransfer {
		nftTransferCallArgs = append(nftTransferCallArgs, vmInput.Arguments[4:]...)
	}

	isSCCallAfter := len(vmInput.Arguments) > vmcommon.MinLenArgumentsDCTNFTTransfer && vmcommon.IsSmartContractAddress(dstAddress)

	if e.shardCoordinator.SelfId() != e.shardCoordinator.ComputeId(dstAddress) {
		gasToTransfer := uint64(0)
		if isSCCallAfter {
			gasToTransfer = vmOutput.GasRemaining
			vmOutput.GasRemaining = 0
		}
		addNFTTransferToVMOutput(
			vmInput.CallerAddr,
			dstAddress,
			vmcommon.BuiltInFunctionDCTNFTTransfer,
			nftTransferCallArgs,
			vmInput.GasLocked,
			gasToTransfer,
			vmInput.CallType,
			vmOutput,
		)

		return nil
	}

	if isSCCallAfter {
		var callArgs [][]byte
		if len(vmInput.Arguments) > vmcommon.MinLenArgumentsDCTNFTTransfer+1 {
			callArgs = vmInput.Arguments[vmcommon.MinLenArgumentsDCTNFTTransfer+1:]
		}

		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			string(vmInput.Arguments[vmcommon.MinLenArgumentsDCTNFTTransfer]),
			callArgs,
			dstAddress,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	return nil
}

func (e *dctNFTTransfer) addNFTToDestination(
	dstAddress []byte,
	userAccount vmcommon.UserAccountHandler,
	dctDataToTransfer *dct.DCToken,
	dctTokenKey []byte,
	mustVerifyPayable bool,
	isReturnWithError bool,
) error {
	if mustVerifyPayable {
		isPayable, errIsPayable := e.payableHandler.IsPayable(dstAddress)
		if errIsPayable != nil {
			return errIsPayable
		}
		if !isPayable {
			return ErrAccountNotPayable
		}
	}

	nonce := uint64(0)
	if dctDataToTransfer.TokenMetaData != nil {
		nonce = dctDataToTransfer.TokenMetaData.Nonce
	}

	currentDCTData, _, err := getDCTNFTTokenOnDestination(userAccount, dctTokenKey, nonce, e.marshalizer)
	if err != nil && !errors.Is(err, ErrNFTTokenDoesNotExist) {
		return err
	}
	err = checkFrozeAndPause(dstAddress, dctTokenKey, currentDCTData, e.pauseHandler, isReturnWithError)
	if err != nil {
		return err
	}

	if currentDCTData.TokenMetaData != nil {
		if !bytes.Equal(currentDCTData.TokenMetaData.Hash, dctDataToTransfer.TokenMetaData.Hash) {
			return ErrWrongNFTOnDestination
		}
	}
	dctDataToTransfer.Value.Add(dctDataToTransfer.Value, currentDCTData.Value)

	_, err = saveDCTNFTToken(userAccount, dctTokenKey, dctDataToTransfer, e.marshalizer, e.pauseHandler, isReturnWithError)
	if err != nil {
		return err
	}

	return nil
}

func addNFTTransferToVMOutput(
	senderAddress []byte,
	recipient []byte,
	funcToCall string,
	arguments [][]byte,
	gasLocked uint64,
	gasLimit uint64,
	callType vmcommon.CallType,
	vmOutput *vmcommon.VMOutput,
) {
	nftTransferTxData := funcToCall
	for _, arg := range arguments {
		nftTransferTxData += "@" + hex.EncodeToString(arg)
	}
	outTransfer := vmcommon.OutputTransfer{
		Value:         big.NewInt(0),
		GasLimit:      gasLimit,
		GasLocked:     gasLocked,
		Data:          []byte(nftTransferTxData),
		CallType:      callType,
		SenderAddress: senderAddress,
	}
	vmOutput.OutputAccounts = make(map[string]*vmcommon.OutputAccount)
	vmOutput.OutputAccounts[string(recipient)] = &vmcommon.OutputAccount{
		Address:         recipient,
		OutputTransfers: []vmcommon.OutputTransfer{outTransfer},
	}
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctNFTTransfer) IsInterfaceNil() bool {
	return e == nil
}
