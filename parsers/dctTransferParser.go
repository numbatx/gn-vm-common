package parsers

import (
	"bytes"
	"math/big"

	vmcommon "github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/check"
	"github.com/numbatx/gn-vm-common/data/dct"
)

// MinArgsForDCTTransfer defines the minimum arguments needed for an dct transfer
const MinArgsForDCTTransfer = 2

// MinArgsForDCTNFTTransfer defines the minimum arguments needed for an nft transfer
const MinArgsForDCTNFTTransfer = 4

// MinArgsForMultiDCTNFTTransfer defines the minimum arguments needed for a multi transfer
const MinArgsForMultiDCTNFTTransfer = 4

// ArgsPerTransfer defines the number of arguments per transfer in multi transfer
const ArgsPerTransfer = 3

type dctTransferParser struct {
	marshalizer vmcommon.Marshalizer
}

// NewDCTTransferParser creates a new dct transfer parser
func NewDCTTransferParser(
	marshalizer vmcommon.Marshalizer,
) (*dctTransferParser, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}

	return &dctTransferParser{marshalizer: marshalizer}, nil
}

// ParseDCTTransfers returns the list of dct transfers, the callFunction and callArgs from the given arguments
func (e *dctTransferParser) ParseDCTTransfers(
	sndAddr []byte,
	rcvAddr []byte,
	function string,
	args [][]byte,
) (*vmcommon.ParsedDCTTransfers, error) {
	switch function {
	case vmcommon.BuiltInFunctionDCTTransfer:
		return e.parseSingleDCTTransfer(rcvAddr, args)
	case vmcommon.BuiltInFunctionDCTNFTTransfer:
		return e.parseSingleDCTNFTTransfer(sndAddr, rcvAddr, args)
	case vmcommon.BuiltInFunctionMultiDCTNFTTransfer:
		return e.parseMultiDCTNFTTransfer(sndAddr, rcvAddr, args)
	default:
		return nil, ErrNotDCTTransferInput
	}
}

func (e *dctTransferParser) parseSingleDCTTransfer(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedDCTTransfers, error) {
	if len(args) < MinArgsForDCTTransfer {
		return nil, ErrNotEnoughArguments
	}
	dctTransfers := &vmcommon.ParsedDCTTransfers{
		DCTTransfers: make([]*vmcommon.DCTTransfer, 1),
		RcvAddr:       rcvAddr,
		CallArgs:      make([][]byte, 0),
		CallFunction:  "",
	}
	if len(args) > MinArgsForDCTTransfer {
		dctTransfers.CallFunction = string(args[MinArgsForDCTTransfer])
	}
	if len(args) > MinArgsForDCTTransfer+1 {
		dctTransfers.CallArgs = append(dctTransfers.CallArgs, args[MinArgsForDCTTransfer+1:]...)
	}
	dctTransfers.DCTTransfers[0] = &vmcommon.DCTTransfer{
		DCTValue:      big.NewInt(0).SetBytes(args[1]),
		DCTTokenName:  args[0],
		DCTTokenType:  uint32(vmcommon.Fungible),
		DCTTokenNonce: 0,
	}

	return dctTransfers, nil
}

func (e *dctTransferParser) parseSingleDCTNFTTransfer(sndAddr, rcvAddr []byte, args [][]byte) (*vmcommon.ParsedDCTTransfers, error) {
	if len(args) < MinArgsForDCTNFTTransfer {
		return nil, ErrNotEnoughArguments
	}
	dctTransfers := &vmcommon.ParsedDCTTransfers{
		DCTTransfers: make([]*vmcommon.DCTTransfer, 1),
		RcvAddr:       rcvAddr,
		CallArgs:      make([][]byte, 0),
		CallFunction:  "",
	}

	if bytes.Equal(sndAddr, rcvAddr) {
		dctTransfers.RcvAddr = args[3]
	}
	if len(args) > MinArgsForDCTNFTTransfer {
		dctTransfers.CallFunction = string(args[MinArgsForDCTNFTTransfer])
	}
	if len(args) > MinArgsForDCTNFTTransfer+1 {
		dctTransfers.CallArgs = append(dctTransfers.CallArgs, args[MinArgsForDCTNFTTransfer+1:]...)
	}
	dctTransfers.DCTTransfers[0] = &vmcommon.DCTTransfer{
		DCTValue:      big.NewInt(0).SetBytes(args[2]),
		DCTTokenName:  args[0],
		DCTTokenType:  uint32(vmcommon.NonFungible),
		DCTTokenNonce: big.NewInt(0).SetBytes(args[1]).Uint64(),
	}

	return dctTransfers, nil
}

func (e *dctTransferParser) parseMultiDCTNFTTransfer(sndAddr, rcvAddr []byte, args [][]byte) (*vmcommon.ParsedDCTTransfers, error) {
	if len(args) < MinArgsForMultiDCTNFTTransfer {
		return nil, ErrNotEnoughArguments
	}
	dctTransfers := &vmcommon.ParsedDCTTransfers{
		RcvAddr:      rcvAddr,
		CallArgs:     make([][]byte, 0),
		CallFunction: "",
	}

	numOfTransfer := big.NewInt(0).SetBytes(args[0])
	startIndex := uint64(1)
	isTxAtSender := false
	if bytes.Equal(sndAddr, rcvAddr) {
		dctTransfers.RcvAddr = args[0]
		numOfTransfer.SetBytes(args[1])
		startIndex = 2
		isTxAtSender = true
	}

	minLenArgs := ArgsPerTransfer*numOfTransfer.Uint64() + startIndex
	if uint64(len(args)) < minLenArgs {
		return nil, ErrNotEnoughArguments
	}

	if uint64(len(args)) > minLenArgs {
		dctTransfers.CallFunction = string(args[minLenArgs])
	}
	if uint64(len(args)) > minLenArgs+1 {
		dctTransfers.CallArgs = append(dctTransfers.CallArgs, args[minLenArgs+1:]...)
	}

	var err error
	dctTransfers.DCTTransfers = make([]*vmcommon.DCTTransfer, numOfTransfer.Uint64())
	for i := uint64(0); i < numOfTransfer.Uint64(); i++ {
		tokenStartIndex := startIndex + i*ArgsPerTransfer
		dctTransfers.DCTTransfers[i], err = e.createNewDCTTransfer(tokenStartIndex, args, isTxAtSender)
		if err != nil {
			return nil, err
		}
	}

	return dctTransfers, nil
}

func (e *dctTransferParser) createNewDCTTransfer(
	tokenStartIndex uint64,
	args [][]byte,
	isTxAtSender bool,
) (*vmcommon.DCTTransfer, error) {
	dctTransfer := &vmcommon.DCTTransfer{
		DCTValue:      big.NewInt(0).SetBytes(args[tokenStartIndex+2]),
		DCTTokenName:  args[tokenStartIndex],
		DCTTokenType:  uint32(vmcommon.Fungible),
		DCTTokenNonce: big.NewInt(0).SetBytes(args[tokenStartIndex+1]).Uint64(),
	}
	if dctTransfer.DCTTokenNonce > 0 {
		dctTransfer.DCTTokenType = uint32(vmcommon.NonFungible)
		if !isTxAtSender {
			transferDCTData := &dct.DCToken{}
			err := e.marshalizer.Unmarshal(transferDCTData, args[tokenStartIndex+2])
			if err != nil {
				return nil, err
			}
			dctTransfer.DCTValue.Set(transferDCTData.Value)
		}
	}

	return dctTransfer, nil
}

// IsInterfaceNil returns true if underlying object is nil
func (e *dctTransferParser) IsInterfaceNil() bool {
	return e == nil
}
