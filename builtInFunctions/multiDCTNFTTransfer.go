package builtInFunctions

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/atomic"
)

type dctNFTMultiTransfer struct {
	*baseEnabled
	keyPrefix             []byte
	marshalizer           vmcommon.Marshalizer
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler
	payableHandler        vmcommon.PayableHandler
	funcGasCost           uint64
	accounts              vmcommon.AccountsAdapter
	shardCoordinator      vmcommon.Coordinator
	gasConfig             vmcommon.BaseOperationCost
	mutExecution          sync.RWMutex

	rolesHandler              vmcommon.DCTRoleHandler
	transferToMetaEnableEpoch uint32
	flagTransferToMeta        atomic.Flag
}

const argumentsPerTransfer = uint64(3)

// NewDCTNFTMultiTransferFunc returns the dct NFT multi transfer built-in function component
func NewDCTNFTMultiTransferFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	accounts vmcommon.AccountsAdapter,
	shardCoordinator vmcommon.Coordinator,
	gasConfig vmcommon.BaseOperationCost,
	activationEpoch uint32,
	epochNotifier vmcommon.EpochNotifier,
	roleHandler vmcommon.DCTRoleHandler,
	transferToMetaEnableEpoch uint32,
) (*dctNFTMultiTransfer, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(globalSettingsHandler) {
		return nil, ErrNilGlobalSettingsHandler
	}
	if check.IfNil(accounts) {
		return nil, ErrNilAccountsAdapter
	}
	if check.IfNil(shardCoordinator) {
		return nil, ErrNilShardCoordinator
	}
	if check.IfNil(epochNotifier) {
		return nil, ErrNilEpochHandler
	}
	if check.IfNil(roleHandler) {
		return nil, ErrNilRolesHandler
	}

	e := &dctNFTMultiTransfer{
		keyPrefix:                 []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier),
		marshalizer:               marshalizer,
		globalSettingsHandler:     globalSettingsHandler,
		funcGasCost:               funcGasCost,
		accounts:                  accounts,
		shardCoordinator:          shardCoordinator,
		gasConfig:                 gasConfig,
		mutExecution:              sync.RWMutex{},
		payableHandler:            &disabledPayableHandler{},
		rolesHandler:              roleHandler,
		transferToMetaEnableEpoch: transferToMetaEnableEpoch,
	}

	e.baseEnabled = &baseEnabled{
		function:        core.BuiltInFunctionMultiDCTNFTTransfer,
		activationEpoch: activationEpoch,
		flagActivated:   atomic.Flag{},
	}

	epochNotifier.RegisterNotifyHandler(e)

	return e, nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (e *dctNFTMultiTransfer) EpochConfirmed(epoch uint32, nonce uint64) {
	e.baseEnabled.EpochConfirmed(epoch, nonce)
	e.flagTransferToMeta.Toggle(epoch >= e.transferToMetaEnableEpoch)
	log.Debug("DCT NFT transfer to metachain flag", "enabled", e.flagTransferToMeta.IsSet())
}

// SetPayableHandler will set the payable handler to the function
func (e *dctNFTMultiTransfer) SetPayableHandler(payableHandler vmcommon.PayableHandler) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctNFTMultiTransfer) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTNFTMultiTransfer
	e.gasConfig = gasCost.BaseOperationCost
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT NFT transfer roles function call
// Requires the following arguments:
// arg0 - destination address
// arg1 - number of tokens to transfer
// list of (tokenID - nonce - quantity) - in case of DCT nonce == 0
// function and list of arguments for SC Call
// if cross-shard, the rest of arguments will be filled inside the SCR
// arg0 - number of tokens to transfer
// list of (tokenID - nonce - quantity/DCT NFT data)
// function and list of arguments for SC Call
func (e *dctNFTMultiTransfer) ProcessBuiltinFunction(
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
		return e.processDCTNFTMultiTransferOnSenderShard(acntSnd, vmInput)
	}

	// in cross shard NFT transfer the sender account must be nil
	if !check.IfNil(acntSnd) {
		return nil, ErrInvalidRcvAddr
	}
	if check.IfNil(acntDst) {
		return nil, ErrInvalidRcvAddr
	}

	numOfTransfers := big.NewInt(0).SetBytes(vmInput.Arguments[0]).Uint64()
	if numOfTransfers == 0 {
		return nil, fmt.Errorf("%w, 0 tokens to transfer", ErrInvalidArguments)
	}
	minNumOfArguments := numOfTransfers*argumentsPerTransfer + 1
	if uint64(len(vmInput.Arguments)) < minNumOfArguments {
		return nil, fmt.Errorf("%w, invalid number of arguments", ErrInvalidArguments)
	}

	verifyPayable := mustVerifyPayable(vmInput, int(minNumOfArguments))
	vmOutput := &vmcommon.VMOutput{GasRemaining: vmInput.GasProvided}
	vmOutput.Logs = make([]*vmcommon.LogEntry, 0, numOfTransfers)
	startIndex := uint64(1)

	err = e.checkIfPayable(verifyPayable, vmInput.RecipientAddr)
	if err != nil {
		return nil, err
	}

	for i := uint64(0); i < numOfTransfers; i++ {
		tokenStartIndex := startIndex + i*argumentsPerTransfer
		tokenID := vmInput.Arguments[tokenStartIndex]
		nonce := big.NewInt(0).SetBytes(vmInput.Arguments[tokenStartIndex+1]).Uint64()

		dctTokenKey := append(e.keyPrefix, tokenID...)

		value := big.NewInt(0)
		if nonce > 0 {
			marshaledNFTTransfer := vmInput.Arguments[tokenStartIndex+2]
			dctTransferData := &dct.DCToken{}
			err = e.marshalizer.Unmarshal(dctTransferData, marshaledNFTTransfer)
			if err != nil {
				return nil, fmt.Errorf("%w for token %s", err, string(tokenID))
			}

			err = e.addNFTToDestination(
				vmInput.RecipientAddr,
				acntDst,
				dctTransferData,
				dctTokenKey,
				vmInput.ReturnCallAfterError)
			if err != nil {
				return nil, fmt.Errorf("%w for token %s", err, string(tokenID))
			}
			value = dctTransferData.Value
		} else {
			transferredValue := big.NewInt(0).SetBytes(vmInput.Arguments[tokenStartIndex+2])
			err = addToDCTBalance(acntDst, dctTokenKey, transferredValue, e.marshalizer, e.globalSettingsHandler, vmInput.ReturnCallAfterError)
			if err != nil {
				return nil, fmt.Errorf("%w for token %s", err, string(tokenID))
			}
			value = transferredValue
		}

		addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionMultiDCTNFTTransfer), tokenID, nonce, value, vmInput.CallerAddr, acntDst.AddressBytes())
	}

	// no need to consume gas on destination - sender already paid for it
	if len(vmInput.Arguments) > int(minNumOfArguments) && vmcommon.IsSmartContractAddress(vmInput.RecipientAddr) {
		var callArgs [][]byte
		if len(vmInput.Arguments) > int(minNumOfArguments)+1 {
			callArgs = vmInput.Arguments[minNumOfArguments+1:]
		}

		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			string(vmInput.Arguments[minNumOfArguments]),
			callArgs,
			vmInput.RecipientAddr,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	return vmOutput, nil
}

func (e *dctNFTMultiTransfer) processDCTNFTMultiTransferOnSenderShard(
	acntSnd vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	dstAddress := vmInput.Arguments[0]
	if len(dstAddress) != len(vmInput.CallerAddr) {
		return nil, fmt.Errorf("%w, not a valid destination address", ErrInvalidArguments)
	}
	if bytes.Equal(dstAddress, vmInput.CallerAddr) {
		return nil, fmt.Errorf("%w, can not transfer to self", ErrInvalidArguments)
	}
	isInvalidTransferToMeta := e.shardCoordinator.ComputeId(dstAddress) == core.MetachainShardId && !e.flagTransferToMeta.IsSet()
	if isInvalidTransferToMeta {
		return nil, ErrInvalidRcvAddr
	}
	numOfTransfers := big.NewInt(0).SetBytes(vmInput.Arguments[1]).Uint64()
	if numOfTransfers == 0 {
		return nil, fmt.Errorf("%w, 0 tokens to transfer", ErrInvalidArguments)
	}
	minNumOfArguments := numOfTransfers*argumentsPerTransfer + 2
	if uint64(len(vmInput.Arguments)) < minNumOfArguments {
		return nil, fmt.Errorf("%w, invalid number of arguments", ErrInvalidArguments)
	}

	multiTransferCost := numOfTransfers * e.funcGasCost
	if vmInput.GasProvided < multiTransferCost {
		return nil, ErrNotEnoughGas
	}

	verifyPayable := mustVerifyPayable(vmInput, int(minNumOfArguments))
	acntDst, err := e.loadAccountIfInShard(dstAddress)
	if err != nil {
		return nil, err
	}

	if !check.IfNil(acntDst) {
		err = e.checkIfPayable(verifyPayable, dstAddress)
		if err != nil {
			return nil, err
		}
	}

	vmOutput := &vmcommon.VMOutput{
		ReturnCode:   vmcommon.Ok,
		GasRemaining: vmInput.GasProvided - multiTransferCost,
		Logs:         make([]*vmcommon.LogEntry, 0, numOfTransfers),
	}

	startIndex := uint64(2)
	listDctData := make([]*dct.DCToken, numOfTransfers)
	listTokenID := make([][]byte, numOfTransfers)
	for i := uint64(0); i < numOfTransfers; i++ {
		tokenStartIndex := startIndex + i*argumentsPerTransfer
		listTokenID[i] = vmInput.Arguments[tokenStartIndex]
		nonce := big.NewInt(0).SetBytes(vmInput.Arguments[tokenStartIndex+1]).Uint64()
		quantityToTransfer := big.NewInt(0).SetBytes(vmInput.Arguments[tokenStartIndex+2])
		listDctData[i], err = e.transferOneTokenOnSenderShard(
			acntSnd,
			acntDst,
			dstAddress,
			listTokenID[i],
			nonce,
			quantityToTransfer,
			vmInput.ReturnCallAfterError)
		if err != nil {
			return nil, fmt.Errorf("%w for token %s", err, string(listTokenID[i]))
		}

		addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionMultiDCTNFTTransfer), listTokenID[i], nonce, quantityToTransfer, vmInput.CallerAddr, dstAddress)
	}

	if !check.IfNil(acntDst) {
		err = e.accounts.SaveAccount(acntDst)
		if err != nil {
			return nil, err
		}
	}

	err = e.createDCTNFTOutputTransfers(vmInput, vmOutput, listDctData, listTokenID, dstAddress)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

func (e *dctNFTMultiTransfer) transferOneTokenOnSenderShard(
	acntSnd vmcommon.UserAccountHandler,
	acntDst vmcommon.UserAccountHandler,
	dstAddress []byte,
	tokenID []byte,
	nonce uint64,
	quantityToTransfer *big.Int,
	isReturnCallWithError bool,
) (*dct.DCToken, error) {
	if quantityToTransfer.Cmp(zero) <= 0 {
		return nil, ErrInvalidNFTQuantity
	}

	dctTokenKey := append(e.keyPrefix, tokenID...)
	dctData, err := getDCTNFTTokenOnSender(acntSnd, dctTokenKey, nonce, e.marshalizer)
	if err != nil {
		return nil, err
	}

	if dctData.Value.Cmp(quantityToTransfer) < 0 {
		return nil, computeInsufficientQuantityDCTError(tokenID, nonce)
	}
	dctData.Value.Sub(dctData.Value, quantityToTransfer)

	_, err = saveDCTNFTToken(acntSnd, dctTokenKey, dctData, e.marshalizer, e.globalSettingsHandler, isReturnCallWithError)
	if err != nil {
		return nil, err
	}

	dctData.Value.Set(quantityToTransfer)

	err = checkIfTransferCanHappenWithLimitedTransfer(dctTokenKey, e.globalSettingsHandler, e.rolesHandler, acntSnd, acntDst, isReturnCallWithError)
	if err != nil {
		return nil, err
	}

	if !check.IfNil(acntDst) {
		if nonce > 0 {
			err = e.addNFTToDestination(dstAddress, acntDst, dctData, dctTokenKey, isReturnCallWithError)
		} else {
			err = addToDCTBalance(acntDst, dctTokenKey, dctData.Value, e.marshalizer, e.globalSettingsHandler, isReturnCallWithError)
		}
		if err != nil {
			return nil, err
		}
	}

	return dctData, nil
}

func computeInsufficientQuantityDCTError(tokenID []byte, nonce uint64) error {
	err := fmt.Errorf("%w for token: %s", ErrInsufficientQuantityDCT, string(tokenID))
	if nonce > 0 {
		err = fmt.Errorf("%w nonce %d", err, nonce)
	}

	return err
}

func (e *dctNFTMultiTransfer) loadAccountIfInShard(dstAddress []byte) (vmcommon.UserAccountHandler, error) {
	if e.shardCoordinator.SelfId() != e.shardCoordinator.ComputeId(dstAddress) {
		return nil, nil
	}

	accountHandler, errLoad := e.accounts.LoadAccount(dstAddress)
	if errLoad != nil {
		return nil, errLoad
	}
	userAccount, ok := accountHandler.(vmcommon.UserAccountHandler)
	if !ok {
		return nil, ErrWrongTypeAssertion
	}

	return userAccount, nil
}

func (e *dctNFTMultiTransfer) createDCTNFTOutputTransfers(
	vmInput *vmcommon.ContractCallInput,
	vmOutput *vmcommon.VMOutput,
	listDCTTransferData []*dct.DCToken,
	listTokenIDs [][]byte,
	dstAddress []byte,
) error {
	multiTransferCallArgs := make([][]byte, 0, argumentsPerTransfer*uint64(len(listDCTTransferData))+1)
	numTokenTransfer := big.NewInt(int64(len(listDCTTransferData))).Bytes()
	multiTransferCallArgs = append(multiTransferCallArgs, numTokenTransfer)

	for i, dctTransferData := range listDCTTransferData {
		multiTransferCallArgs = append(multiTransferCallArgs, listTokenIDs[i])
		if dctTransferData.TokenMetaData != nil {
			marshaledNFTTransfer, err := e.marshalizer.Marshal(dctTransferData)
			if err != nil {
				return err
			}

			gasForTransfer := uint64(len(marshaledNFTTransfer)) * e.gasConfig.DataCopyPerByte
			if gasForTransfer > vmOutput.GasRemaining {
				return ErrNotEnoughGas
			}
			vmOutput.GasRemaining -= gasForTransfer

			multiTransferCallArgs = append(multiTransferCallArgs, big.NewInt(0).SetUint64(dctTransferData.TokenMetaData.Nonce).Bytes())
			multiTransferCallArgs = append(multiTransferCallArgs, marshaledNFTTransfer)
		} else {
			multiTransferCallArgs = append(multiTransferCallArgs, []byte{0})
			multiTransferCallArgs = append(multiTransferCallArgs, dctTransferData.Value.Bytes())
		}
	}

	minNumOfArguments := uint64(len(listDCTTransferData))*argumentsPerTransfer + 2
	if uint64(len(vmInput.Arguments)) > minNumOfArguments {
		multiTransferCallArgs = append(multiTransferCallArgs, vmInput.Arguments[minNumOfArguments:]...)
	}

	isSCCallAfter := determineIsSCCallAfter(vmInput, dstAddress, int(minNumOfArguments))

	if e.shardCoordinator.SelfId() != e.shardCoordinator.ComputeId(dstAddress) {
		gasToTransfer := uint64(0)
		if isSCCallAfter {
			gasToTransfer = vmOutput.GasRemaining
			vmOutput.GasRemaining = 0
		}
		addNFTTransferToVMOutput(
			vmInput.CallerAddr,
			dstAddress,
			core.BuiltInFunctionMultiDCTNFTTransfer,
			multiTransferCallArgs,
			vmInput.GasLocked,
			gasToTransfer,
			vmInput.CallType,
			vmOutput,
		)

		return nil
	}

	if isSCCallAfter {
		var callArgs [][]byte
		if uint64(len(vmInput.Arguments)) > minNumOfArguments+1 {
			callArgs = vmInput.Arguments[minNumOfArguments+1:]
		}

		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			string(vmInput.Arguments[minNumOfArguments]),
			callArgs,
			dstAddress,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	return nil
}

func (e *dctNFTMultiTransfer) checkIfPayable(
	mustVerifyPayable bool,
	dstAddress []byte,
) error {
	if !mustVerifyPayable {
		return nil
	}

	isPayable, errIsPayable := e.payableHandler.IsPayable(dstAddress)
	if errIsPayable != nil {
		return errIsPayable
	}
	if !isPayable {
		return ErrAccountNotPayable
	}

	return nil
}

func (e *dctNFTMultiTransfer) addNFTToDestination(
	dstAddress []byte,
	userAccount vmcommon.UserAccountHandler,
	dctDataToTransfer *dct.DCToken,
	dctTokenKey []byte,
	isReturnCallWithError bool,
) error {
	nonce := uint64(0)
	if dctDataToTransfer.TokenMetaData != nil {
		nonce = dctDataToTransfer.TokenMetaData.Nonce
	}

	currentDCTData, _, err := getDCTNFTTokenOnDestination(userAccount, dctTokenKey, nonce, e.marshalizer)
	if err != nil && !errors.Is(err, ErrNFTTokenDoesNotExist) {
		return err
	}
	err = checkFrozeAndPause(dstAddress, dctTokenKey, currentDCTData, e.globalSettingsHandler, isReturnCallWithError)
	if err != nil {
		return err
	}

	if currentDCTData.TokenMetaData != nil {
		if !bytes.Equal(currentDCTData.TokenMetaData.Hash, dctDataToTransfer.TokenMetaData.Hash) {
			return ErrWrongNFTOnDestination
		}
		dctDataToTransfer.Value.Add(dctDataToTransfer.Value, currentDCTData.Value)
	}

	_, err = saveDCTNFTToken(userAccount, dctTokenKey, dctDataToTransfer, e.marshalizer, e.globalSettingsHandler, isReturnCallWithError)
	if err != nil {
		return err
	}

	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctNFTMultiTransfer) IsInterfaceNil() bool {
	return e == nil
}
