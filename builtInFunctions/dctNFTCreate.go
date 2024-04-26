package builtInFunctions

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	"github.com/numbatx/gn-core/data/dct"
	"github.com/numbatx/gn-vm-common"
)

var noncePrefix = []byte(core.NumbatProtectedKeyPrefix + core.DCTNFTLatestNonceIdentifier)

type dctNFTCreate struct {
	baseAlwaysActive
	keyPrefix             []byte
	marshalizer           vmcommon.Marshalizer
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler
	rolesHandler          vmcommon.DCTRoleHandler
	funcGasCost           uint64
	gasConfig             vmcommon.BaseOperationCost
	mutExecution          sync.RWMutex
}

// NewDCTNFTCreateFunc returns the dct NFT create built-in function component
func NewDCTNFTCreateFunc(
	funcGasCost uint64,
	gasConfig vmcommon.BaseOperationCost,
	marshalizer vmcommon.Marshalizer,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	rolesHandler vmcommon.DCTRoleHandler,
) (*dctNFTCreate, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(globalSettingsHandler) {
		return nil, ErrNilGlobalSettingsHandler
	}
	if check.IfNil(rolesHandler) {
		return nil, ErrNilRolesHandler
	}

	e := &dctNFTCreate{
		keyPrefix:             []byte(core.NumbatProtectedKeyPrefix + core.DCTKeyIdentifier),
		marshalizer:           marshalizer,
		globalSettingsHandler: globalSettingsHandler,
		rolesHandler:          rolesHandler,
		funcGasCost:           funcGasCost,
		gasConfig:             gasConfig,
		mutExecution:          sync.RWMutex{},
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctNFTCreate) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.DCTNFTCreate
	e.gasConfig = gasCost.BaseOperationCost
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves DCT NFT create function call
// Requires at least 7 arguments:
// arg0 - token identifier
// arg1 - initial quantity
// arg2 - NFT name
// arg3 - Royalties - max 10000
// arg4 - hash
// arg5 - attributes
// arg6+ - multiple entries of URI (minimum 1)
func (e *dctNFTCreate) ProcessBuiltinFunction(
	acntSnd, _ vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	err := checkDCTNFTCreateBurnAddInput(acntSnd, vmInput, e.funcGasCost)
	if err != nil {
		return nil, err
	}
	if len(vmInput.Arguments) < 7 {
		return nil, fmt.Errorf("%w, wrong number of arguments", ErrInvalidArguments)
	}

	tokenID := vmInput.Arguments[0]
	err = e.rolesHandler.CheckAllowedToExecute(acntSnd, vmInput.Arguments[0], []byte(core.DCTRoleNFTCreate))
	if err != nil {
		return nil, err
	}

	nonce, err := getLatestNonce(acntSnd, tokenID)
	if err != nil {
		return nil, err
	}

	totalLength := uint64(0)
	for _, arg := range vmInput.Arguments {
		totalLength += uint64(len(arg))
	}
	gasToUse := totalLength*e.gasConfig.StorePerByte + e.funcGasCost
	if vmInput.GasProvided < gasToUse {
		return nil, ErrNotEnoughGas
	}

	royalties := uint32(big.NewInt(0).SetBytes(vmInput.Arguments[3]).Uint64())
	if royalties > core.MaxRoyalty {
		return nil, fmt.Errorf("%w, invalid max royality value", ErrInvalidArguments)
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)
	quantity := big.NewInt(0).SetBytes(vmInput.Arguments[1])
	if quantity.Cmp(zero) <= 0 {
		return nil, fmt.Errorf("%w, invalid quantity", ErrInvalidArguments)
	}
	if quantity.Cmp(big.NewInt(1)) > 0 {
		err = e.rolesHandler.CheckAllowedToExecute(acntSnd, vmInput.Arguments[0], []byte(core.DCTRoleNFTAddQuantity))
		if err != nil {
			return nil, err
		}
	}

	nextNonce := nonce + 1
	dctData := &dct.DCToken{
		Type:  uint32(core.NonFungible),
		Value: quantity,
		TokenMetaData: &dct.MetaData{
			Nonce:      nextNonce,
			Name:       vmInput.Arguments[2],
			Creator:    vmInput.CallerAddr,
			Royalties:  royalties,
			Hash:       vmInput.Arguments[4],
			Attributes: vmInput.Arguments[5],
			URIs:       vmInput.Arguments[6:],
		},
	}

	var dctDataBytes []byte
	dctDataBytes, err = saveDCTNFTToken(acntSnd, dctTokenKey, dctData, e.marshalizer, e.globalSettingsHandler, vmInput.ReturnCallAfterError)
	if err != nil {
		return nil, err
	}

	err = saveLatestNonce(acntSnd, tokenID, nextNonce)
	if err != nil {
		return nil, err
	}

	vmOutput := &vmcommon.VMOutput{
		ReturnCode:   vmcommon.Ok,
		GasRemaining: vmInput.GasProvided - gasToUse,
		ReturnData:   [][]byte{big.NewInt(0).SetUint64(nextNonce).Bytes()},
	}

	addDCTEntryInVMOutput(vmOutput, []byte(core.BuiltInFunctionDCTNFTCreate), vmInput.Arguments[0], nextNonce, quantity, vmInput.CallerAddr, dctDataBytes)

	return vmOutput, nil
}

func getLatestNonce(acnt vmcommon.UserAccountHandler, tokenID []byte) (uint64, error) {
	nonceKey := getNonceKey(tokenID)
	nonceData, err := acnt.AccountDataHandler().RetrieveValue(nonceKey)
	if err != nil {
		return 0, err
	}

	if len(nonceData) == 0 {
		return 0, nil
	}

	return big.NewInt(0).SetBytes(nonceData).Uint64(), nil
}

func saveLatestNonce(acnt vmcommon.UserAccountHandler, tokenID []byte, nonce uint64) error {
	nonceKey := getNonceKey(tokenID)
	return acnt.AccountDataHandler().SaveKeyValue(nonceKey, big.NewInt(0).SetUint64(nonce).Bytes())
}

func computeDCTNFTTokenKey(dctTokenKey []byte, nonce uint64) []byte {
	return append(dctTokenKey, big.NewInt(0).SetUint64(nonce).Bytes()...)
}

func getDCTNFTTokenOnSender(
	accnt vmcommon.UserAccountHandler,
	dctTokenKey []byte,
	nonce uint64,
	marshalizer vmcommon.Marshalizer,
) (*dct.DCToken, error) {
	dctData, isNew, err := getDCTNFTTokenOnDestination(accnt, dctTokenKey, nonce, marshalizer)
	if err != nil {
		return nil, err
	}
	if isNew {
		return nil, ErrNewNFTDataOnSenderAddress
	}

	return dctData, nil
}

func getDCTNFTTokenOnDestination(
	accnt vmcommon.UserAccountHandler,
	dctTokenKey []byte,
	nonce uint64,
	marshalizer vmcommon.Marshalizer,
) (*dct.DCToken, bool, error) {
	dctNFTTokenKey := computeDCTNFTTokenKey(dctTokenKey, nonce)
	dctData := &dct.DCToken{Value: big.NewInt(0), Type: uint32(core.Fungible)}
	marshaledData, err := accnt.AccountDataHandler().RetrieveValue(dctNFTTokenKey)
	if err != nil || len(marshaledData) == 0 {
		return dctData, true, nil
	}

	err = marshalizer.Unmarshal(dctData, marshaledData)
	if err != nil {
		return nil, false, err
	}

	return dctData, false, nil
}

func saveDCTNFTToken(
	acnt vmcommon.UserAccountHandler,
	dctTokenKey []byte,
	dctData *dct.DCToken,
	marshalizer vmcommon.Marshalizer,
	globalSettingsHandler vmcommon.DCTGlobalSettingsHandler,
	isReturnWithError bool,
) ([]byte, error) {
	err := checkFrozeAndPause(acnt.AddressBytes(), dctTokenKey, dctData, globalSettingsHandler, isReturnWithError)
	if err != nil {
		return nil, err
	}

	nonce := uint64(0)
	if dctData.TokenMetaData != nil {
		nonce = dctData.TokenMetaData.Nonce
	}
	dctNFTTokenKey := computeDCTNFTTokenKey(dctTokenKey, nonce)
	err = checkFrozeAndPause(acnt.AddressBytes(), dctNFTTokenKey, dctData, globalSettingsHandler, isReturnWithError)
	if err != nil {
		return nil, err
	}

	if dctData.Value.Cmp(zero) <= 0 {
		return nil, acnt.AccountDataHandler().SaveKeyValue(dctNFTTokenKey, nil)
	}

	marshaledData, err := marshalizer.Marshal(dctData)
	if err != nil {
		return nil, err
	}

	return marshaledData, acnt.AccountDataHandler().SaveKeyValue(dctNFTTokenKey, marshaledData)
}

func checkDCTNFTCreateBurnAddInput(
	account vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
	funcGasCost uint64,
) error {
	err := checkBasicDCTArguments(vmInput)
	if err != nil {
		return err
	}
	if !bytes.Equal(vmInput.CallerAddr, vmInput.RecipientAddr) {
		return ErrInvalidRcvAddr
	}
	if check.IfNil(account) {
		return ErrNilUserAccount
	}
	if vmInput.GasProvided < funcGasCost {
		return ErrNotEnoughGas
	}
	return nil
}

func getNonceKey(tokenID []byte) []byte {
	return append(noncePrefix, tokenID...)
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctNFTCreate) IsInterfaceNil() bool {
	return e == nil
}
