package builtInFunctions

import (
	"math/big"
	"testing"

	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/data/dct"
	"github.com/numbatx/gn-vm-common/mock"
	"github.com/stretchr/testify/assert"
)

func TestDCTFreezeWipe_ProcessBuiltInFunctionErrors(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	freeze, _ := NewDCTFreezeWipeFunc(marshalizer, true, false)
	_, err := freeze.ProcessBuiltinFunction(nil, nil, nil)
	assert.Equal(t, err, ErrNilVmInput)

	input := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue: big.NewInt(0),
		},
	}
	_, err = freeze.ProcessBuiltinFunction(nil, nil, input)
	assert.Equal(t, err, ErrInvalidArguments)

	input = &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			GasProvided: 50,
			CallValue:   big.NewInt(1),
		},
	}
	_, err = freeze.ProcessBuiltinFunction(nil, nil, input)
	assert.Equal(t, err, ErrBuiltInFunctionCalledWithValue)

	input.CallValue = big.NewInt(0)
	key := []byte("key")
	value := []byte("value")
	input.Arguments = [][]byte{key, value}
	_, err = freeze.ProcessBuiltinFunction(nil, nil, input)
	assert.Equal(t, err, ErrInvalidArguments)

	input.Arguments = [][]byte{key}
	_, err = freeze.ProcessBuiltinFunction(nil, nil, input)
	assert.Equal(t, err, ErrAddressIsNotDCTSystemSC)

	input.CallerAddr = vmcommon.DCTSCAddress
	_, err = freeze.ProcessBuiltinFunction(nil, nil, input)
	assert.Equal(t, err, ErrNilUserAccount)

	input.RecipientAddr = []byte("dst")
	acnt := mock.NewUserAccount(input.RecipientAddr)
	_, err = freeze.ProcessBuiltinFunction(nil, acnt, input)
	assert.Nil(t, err)

	dctToken := &dct.DCToken{}
	dctKey := append(freeze.keyPrefix, key...)
	marshaledData, _ := acnt.AccountDataHandler().RetrieveValue(dctKey)
	_ = marshalizer.Unmarshal(dctToken, marshaledData)

	dctUserData := DCTUserMetadataFromBytes(dctToken.Properties)
	assert.True(t, dctUserData.Frozen)
}

func TestDCTFreezeWipe_ProcessBuiltInFunction(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	freeze, _ := NewDCTFreezeWipeFunc(marshalizer, true, false)
	_, err := freeze.ProcessBuiltinFunction(nil, nil, nil)
	assert.Equal(t, err, ErrNilVmInput)

	input := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallValue: big.NewInt(0),
		},
	}
	key := []byte("key")

	input.Arguments = [][]byte{key}
	input.CallerAddr = vmcommon.DCTSCAddress
	input.RecipientAddr = []byte("dst")
	dctKey := append(freeze.keyPrefix, key...)
	dctToken := &dct.DCToken{Value: big.NewInt(10)}
	marshaledData, _ := freeze.marshalizer.Marshal(dctToken)
	acnt := mock.NewUserAccount(input.RecipientAddr)
	_ = acnt.AccountDataHandler().SaveKeyValue(dctKey, marshaledData)

	_, err = freeze.ProcessBuiltinFunction(nil, acnt, input)
	assert.Nil(t, err)

	dctToken = &dct.DCToken{}
	marshaledData, _ = acnt.AccountDataHandler().RetrieveValue(dctKey)
	_ = marshalizer.Unmarshal(dctToken, marshaledData)

	dctUserData := DCTUserMetadataFromBytes(dctToken.Properties)
	assert.True(t, dctUserData.Frozen)

	unFreeze, _ := NewDCTFreezeWipeFunc(marshalizer, false, false)
	_, err = unFreeze.ProcessBuiltinFunction(nil, acnt, input)
	assert.Nil(t, err)

	marshaledData, _ = acnt.AccountDataHandler().RetrieveValue(dctKey)
	_ = marshalizer.Unmarshal(dctToken, marshaledData)

	dctUserData = DCTUserMetadataFromBytes(dctToken.Properties)
	assert.False(t, dctUserData.Frozen)

	// cannot wipe if account is not frozen
	wipe, _ := NewDCTFreezeWipeFunc(marshalizer, false, true)
	_, err = wipe.ProcessBuiltinFunction(nil, acnt, input)
	assert.Equal(t, ErrCannotWipeAccountNotFrozen, err)

	marshaledData, _ = acnt.AccountDataHandler().RetrieveValue(dctKey)
	assert.NotEqual(t, 0, len(marshaledData))

	// can wipe as account is frozen
	metaData := DCTUserMetadata{Frozen: true}
	dctToken = &dct.DCToken{
		Properties: metaData.ToBytes(),
	}
	dctTokenBytes, _ := marshalizer.Marshal(dctToken)
	err = acnt.AccountDataHandler().SaveKeyValue(dctKey, dctTokenBytes)
	assert.NoError(t, err)

	wipe, _ = NewDCTFreezeWipeFunc(marshalizer, false, true)
	_, err = wipe.ProcessBuiltinFunction(nil, acnt, input)
	assert.NoError(t, err)

	marshaledData, _ = acnt.AccountDataHandler().RetrieveValue(dctKey)
	assert.Equal(t, 0, len(marshaledData))
}
