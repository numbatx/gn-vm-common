package builtInFunctions

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/atomic"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-core/data/vm"
	"github.com/numbatx/gn-vm-common"
)

var oneValue = big.NewInt(1)
var zeroByteArray = []byte{0}

type dctNFTTransfer struct {
	baseAlwaysActive
	keyPrefix                 []byte
	marshalizer               vmcommon.Marshalizer
	globalSettingsHandler     vmcommon.DCTGlobalSettingsHandler
	payableHandler            vmcommon.PayableHandler
	funcGasCost               uint64
	accounts                  vmcommon.AccountsAdapter
	shardCoordinator          vmcommon.Coordinator
	gasConfig                 vmcommon.BaseOperationCost
	mutExecution              sync.RWMutex
	rolesHandler              vmcommon.DCTRoleHandler
	dctStorageHandler        vmcommon.DCTNFTStorageHandler
	transferToMetaEnableEpoch uint32
	flagTransferToMeta        atomic.Flag
	check0TransferEnableEpoch uint32
	flagCheck0Transfer        atomic.Flag
}

// NewDCTNFTTransferFunc returns the dct NFT transfer built-in function component
func NewDCTNFTTransferFunc(
	funcGasCost uint64,
	marshalizer vmcommon.Marshalizer,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	accounts vmcommon.AccountsAdapter,
	shardCoordinator vmcommon.Coordinator,
	gasConfig vmcommon.BaseOperationCost,
	rolesHandler vmcommon.DCTRoleHandler,
	transferToMetaEnableEpoch uint32,
	checkZeroTransferEnableEpoch uint32,
	dctStorageHandler vmcommon.DCTNFTStorageHandler,
	epochNotifier vmcommon.EpochNotifier,
) (*dctNFTTransfer, error) {
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
	if check.IfNil(rolesHandler) {
		return nil, ErrNilRolesHandler
	}
	if check.IfNil(epochNotifier) {
		return nil, ErrNilEpochHandler
	}
	if check.IfNil(dctStorageHandler) {
		return nil, ErrNilDCTNFTStorageHandler
	}

	e := &dctNFTTransfer{
		keyPrefix:                 []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier),
		marshalizer:               marshalizer,
		globalSettingsHandler:     globalSettingsHandler,
		funcGasCost:               funcGasCost,
		accounts:                  accounts,
		shardCoordinator:          shardCoordinator,
		gasConfig:                 gasConfig,
		mutExecution:              sync.RWMutex{},
		payableHandler:            &disabledPayableHandler{},
		rolesHandler:              rolesHandler,
		transferToMetaEnableEpoch: transferToMetaEnableEpoch,
		check0TransferEnableEpoch: checkZeroTransferEnableEpoch,
		dctStorageHandler:        dctStorageHandler,
	}

	epochNotifier.RegisterNotifyHandler(e)

	return e, nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (e *dctNFTTransfer) EpochConfirmed(epoch uint32, _ uint64) {
	e.flagTransferToMeta.SetValue(epoch >= e.transferToMetaEnableEpoch)
	log.Debug("DCT NFT transfer to metachain flag", "enabled", e.flagTransferToMeta.IsSet())
	e.flagCheck0Transfer.SetValue(epoch >= e.check0TransferEnableEpoch)
	log.Debug("DCT NFT transfer check zero transfer", "enabled", e.flagCheck0Transfer.IsSet())
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

	tickerID := vmInput.Arguments[0]
	dctTokenKey := append(e.keyPrefix, tickerID...)
	nonce := big.NewInt(0).SetBytes(vmInput.Arguments[1]).Uint64()
	value := big.NewInt(0).SetBytes(vmInput.Arguments[2])

	dctTransferData := &dct.DCToken{}
	if !bytes.Equal(vmInput.Arguments[3], zeroByteArray) {
		marshaledNFTTransfer := vmInput.Arguments[3]
		err = e.marshalizer.Unmarshal(dctTransferData, marshaledNFTTransfer)
		if err != nil {
			return nil, err
		}
	} else {
		dctTransferData.Value = big.NewInt(0).Set(value)
		dctTransferData.Type = uint32(core.NonFungible)
	}

	err = e.addNFTToDestination(vmInput.CallerAddr, vmInput.RecipientAddr, acntDst, dctTransferData, dctTokenKey, nonce, mustVerifyPayable(vmInput, core.MinLenArgumentsDCTNFTTransfer), vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	// no need to consume gas on destination - sender already paid for it
	vmOutput := &vmcommon.VMOutput{GasRemaining: vmInput.GasProvided}
	if len(vmInput.Arguments) > core.MinLenArgumentsDCTNFTTransfer && vmcommon.IsSmartContractAddress(vmInput.RecipientAddr) {
		var callArgs [][]byte
		if len(vmInput.Arguments) > core.MinLenArgumentsDCTNFTTransfer+1 {
			callArgs = vmInput.Arguments[core.MinLenArgumentsDCTNFTTransfer+1:]
		}

		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			string(vmInput.Arguments[core.MinLenArgumentsDCTNFTTransfer]),
			callArgs,
			vmInput.RecipientAddr,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTNFTTransfer), vmInput.Arguments[0], nonce, value, vmInput.CallerAddr, acntDst.AddressBytes())

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
	isInvalidTransferToMeta := e.shardCoordinator.ComputeId(dstAddress) == core.MetachainShardId && !e.flagTransferToMeta.IsSet()
	if isInvalidTransferToMeta {
		return nil, ErrInvalidRcvAddr
	}
	if vmInput.GasProvided < e.funcGasCost {
		return nil, ErrNotEnoughGas
	}

	tickerID := vmInput.Arguments[0]
	dctTokenKey := append(e.keyPrefix, tickerID...)
	nonce := big.NewInt(0).SetBytes(vmInput.Arguments[1]).Uint64()
	dctData, err := e.dctStorageHandler.GetDCTNFTTokenOnSender(acntSnd, dctTokenKey, nonce)
	if err != nil {
		return nil, err
	}
	if nonce == 0 {
		return nil, ErrNFTDoesNotHaveMetadata
	}

	quantityToTransfer := big.NewInt(0).SetBytes(vmInput.Arguments[2])
	if dctData.Value.Cmp(quantityToTransfer) < 0 {
		return nil, ErrInvalidNFTQuantity
	}
	if e.flagCheck0Transfer.IsSet() && quantityToTransfer.Cmp(zero) <= 0 {
		return nil, ErrInvalidNFTQuantity
	}
	dctData.Value.Sub(dctData.Value, quantityToTransfer)

	_, err = e.dctStorageHandler.SaveDCTNFTToken(acntSnd.AddressBytes(), acntSnd, dctTokenKey, nonce, dctData, false, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	dctData.Value.Set(quantityToTransfer)

	var userAccount vmcommon.UserAccountHandler
	if e.shardCoordinator.SelfId() == e.shardCoordinator.ComputeId(dstAddress) {
		accountHandler, errLoad := e.accounts.LoadAccount(dstAddress)
		if errLoad != nil {
			return nil, errLoad
		}

		var ok bool
		userAccount, ok = accountHandler.(vmcommon.UserAccountHandler)
		if !ok {
			return nil, ErrWrongTypeAssertion
		}

		err = e.addNFTToDestination(vmInput.CallerAddr, dstAddress, userAccount, dctData, dctTokenKey, nonce, mustVerifyPayable(vmInput, core.MinLenArgumentsDCTNFTTransfer), vmInput.ReturnCallAfterError)
		if err != nil {
			return nil, err
		}

		err = e.accounts.SaveAccount(userAccount)
		if err != nil {
			return nil, err
		}
	}

	err = checkIfTransferCanHappenWithLimitedTransfer(dctTokenKey, e.globalSettingsHandler, e.rolesHandler, acntSnd, userAccount, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	vmOutput := &vmcommon.VMOutput{
		ReturnCode:   vmcommon.Ok,
		GasRemaining: vmInput.GasProvided - e.funcGasCost,
	}
	err = e.createNFTOutputTransfers(vmInput, vmOutput, dctData, dstAddress, tickerID, nonce)
	if err != nil {
		return nil, err
	}

	tokenNonce := dctData.TokenMetaData.Nonce
	addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTNFTTransfer), vmInput.Arguments[0], tokenNonce, quantityToTransfer, vmInput.CallerAddr, dstAddress)

	return vmOutput, nil
}

func (e *dctNFTTransfer) createNFTOutputTransfers(
	vmInput *vmcommon.ContractCallInput,
	vmOutput *vmcommon.VMOutput,
	dctTransferData *dct.DCToken,
	dstAddress []byte,
	tickerID []byte,
	nonce uint64,
) error {
	nftTransferCallArgs := make([][]byte, 0)
	nftTransferCallArgs = append(nftTransferCallArgs, vmInput.Arguments[:3]...)

	wasAlreadySent, err := e.dctStorageHandler.WasAlreadySentToDestinationShardAndUpdateState(tickerID, nonce, dstAddress)
	if err != nil {
		return err
	}

	if !wasAlreadySent || dctTransferData.Value.Cmp(oneValue) == 0 {
		marshaledNFTTransfer, err := e.marshalizer.Marshal(dctTransferData)
		if err != nil {
			return err
		}

		gasForTransfer := uint64(len(marshaledNFTTransfer)) * e.gasConfig.DataCopyPerByte
		if gasForTransfer > vmOutput.GasRemaining {
			return ErrNotEnoughGas
		}
		vmOutput.GasRemaining -= gasForTransfer
		nftTransferCallArgs = append(nftTransferCallArgs, marshaledNFTTransfer)
	} else {
		nftTransferCallArgs = append(nftTransferCallArgs, zeroByteArray)
	}

	if len(vmInput.Arguments) > core.MinLenArgumentsDCTNFTTransfer {
		nftTransferCallArgs = append(nftTransferCallArgs, vmInput.Arguments[4:]...)
	}

	isSCCallAfter := determineIsSCCallAfter(vmInput, dstAddress, core.MinLenArgumentsDCTNFTTransfer)

	if e.shardCoordinator.SelfId() != e.shardCoordinator.ComputeId(dstAddress) {
		gasToTransfer := uint64(0)
		if isSCCallAfter {
			gasToTransfer = vmOutput.GasRemaining
			vmOutput.GasRemaining = 0
		}
		addNFTTransferToVMOutput(
			vmInput.CallerAddr,
			dstAddress,
			core.BuiltInFunctionDCTNFTTransfer,
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
		if len(vmInput.Arguments) > core.MinLenArgumentsDCTNFTTransfer+1 {
			callArgs = vmInput.Arguments[core.MinLenArgumentsDCTNFTTransfer+1:]
		}

		addOutputTransferToVMOutput(
			vmInput.CallerAddr,
			string(vmInput.Arguments[core.MinLenArgumentsDCTNFTTransfer]),
			callArgs,
			dstAddress,
			vmInput.GasLocked,
			vmInput.CallType,
			vmOutput)
	}

	return nil
}

func (e *dctNFTTransfer) addNFTToDestination(
	sndAddress []byte,
	dstAddress []byte,
	userAccount vmcommon.UserAccountHandler,
	dctDataToTransfer *dct.DCToken,
	dctTokenKey []byte,
	nonce uint64,
	mustVerifyPayable bool,
	isReturnWithError bool,
) error {
	if mustVerifyPayable {
		isPayable, errIsPayable := e.payableHandler.IsPayable(sndAddress, dstAddress)
		if errIsPayable != nil {
			return errIsPayable
		}
		if !isPayable {
			return ErrAccountNotPayable
		}
	}

	currentDCTData, _, err := e.dctStorageHandler.GetDCTNFTTokenOnDestination(userAccount, dctTokenKey, nonce)
	if err != nil && !errors.Is(err, ErrNFTTokenDoesNotExist) {
		return err
	}
	err = checkFrozeAndPause(dstAddress, dctTokenKey, currentDCTData, e.globalSettingsHandler, isReturnWithError)
	if err != nil {
		return err
	}

	dctDataToTransfer.Value.Add(dctDataToTransfer.Value, currentDCTData.Value)
	_, err = e.dctStorageHandler.SaveDCTNFTToken(sndAddress, userAccount, dctTokenKey, nonce, dctDataToTransfer, false, isReturnWithError)
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
	callType vm.CallType,
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
