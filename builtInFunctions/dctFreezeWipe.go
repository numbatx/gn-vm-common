package builtInFunctions

import (
	"bytes"
	"math/big"

	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	vmcommon "github.com/numbatx/gn-vm-common"
)

type dctFreezeWipe struct {
	baseAlwaysActiveHandler
	marshaller vmcommon.Marshalizer
	keyPrefix  []byte
	wipe       bool
	freeze     bool
}

// NewDCTFreezeWipeFunc returns the dct freeze/un-freeze/wipe built-in function component
func NewDCTFreezeWipeFunc(
	marshaller vmcommon.Marshalizer,
	freeze bool,
	wipe bool,
) (*dctFreezeWipe, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}

	e := &dctFreezeWipe{
		marshaller: marshaller,
		keyPrefix:  []byte(baseDCTKeyPrefix),
		freeze:     freeze,
		wipe:       wipe,
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctFreezeWipe) SetNewGasConfig(_ *vmcommon.GasCost) {
}

// ProcessBuiltinFunction resolves DCT transfer function call
func (e *dctFreezeWipe) ProcessBuiltinFunction(
	_, acntDst vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	if vmInput == nil {
		return nil, ErrNilVmInput
	}
	if vmInput.CallValue.Cmp(zero) != 0 {
		return nil, ErrBuiltInFunctionCalledWithValue
	}
	if len(vmInput.Arguments) != 1 {
		return nil, ErrInvalidArguments
	}
	if !bytes.Equal(vmInput.CallerAddr, core.DCTSCAddress) {
		return nil, ErrAddressIsNotDCTSystemSC
	}
	if check.IfNil(acntDst) {
		return nil, ErrNilUserAccount
	}

	dctTokenKey := append(e.keyPrefix, vmInput.Arguments[0]...)

	var amount *big.Int
	var err error

	if e.wipe {
		amount, err = e.wipeIfApplicable(acntDst, dctTokenKey)
		if err != nil {
			return nil, err
		}

	} else {
		amount, err = e.toggleFreeze(acntDst, dctTokenKey)
		if err != nil {
			return nil, err
		}
	}

	vmOutput := &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}
	identifier, nonce := extractTokenIdentifierAndNonceDCTWipe(vmInput.Arguments[0])
	addDCTEntryInVMOutput(vmOutput, []byte(vmInput.Function), identifier, nonce, amount, vmInput.CallerAddr, acntDst.AddressBytes())

	return vmOutput, nil
}

func (e *dctFreezeWipe) wipeIfApplicable(acntDst vmcommon.UserAccountHandler, tokenKey []byte) (*big.Int, error) {
	tokenData, err := getDCTDataFromKey(acntDst, tokenKey, e.marshaller)
	if err != nil {
		return nil, err
	}

	dctUserMetadata := DCTUserMetadataFromBytes(tokenData.Properties)
	if !dctUserMetadata.Frozen {
		return nil, ErrCannotWipeAccountNotFrozen
	}

	err = acntDst.AccountDataHandler().SaveKeyValue(tokenKey, nil)
	if err != nil {
		return nil, err
	}

	wipedAmount := vmcommon.ZeroValueIfNil(tokenData.Value)
	return wipedAmount, nil
}

func (e *dctFreezeWipe) toggleFreeze(acntDst vmcommon.UserAccountHandler, tokenKey []byte) (*big.Int, error) {
	tokenData, err := getDCTDataFromKey(acntDst, tokenKey, e.marshaller)
	if err != nil {
		return nil, err
	}

	dctUserMetadata := DCTUserMetadataFromBytes(tokenData.Properties)
	dctUserMetadata.Frozen = e.freeze
	tokenData.Properties = dctUserMetadata.ToBytes()

	err = saveDCTData(acntDst, tokenData, tokenKey, e.marshaller)
	if err != nil {
		return nil, err
	}

	frozenAmount := vmcommon.ZeroValueIfNil(tokenData.Value)
	return frozenAmount, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctFreezeWipe) IsInterfaceNil() bool {
	return e == nil
}
