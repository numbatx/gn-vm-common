package builtInFunctions

import (
	vmcommon "github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/check"
	"github.com/mitchellh/mapstructure"
)

// ArgsCreateBuiltInFunctionContainer -
type ArgsCreateBuiltInFunctionContainer struct {
	GasMap                              map[string]map[string]uint64
	MapDNSAddresses                     map[string]struct{}
	EnableUserNameChange                bool
	Marshalizer                         vmcommon.Marshalizer
	Accounts                            vmcommon.AccountsAdapter
	ShardCoordinator                    vmcommon.Coordinator
	EpochNotifier                       vmcommon.EpochNotifier
	DCTNFTImprovementV1ActivationEpoch uint32
}

type builtInFuncFactory struct {
	mapDNSAddresses                     map[string]struct{}
	enableUserNameChange                bool
	marshalizer                         vmcommon.Marshalizer
	accounts                            vmcommon.AccountsAdapter
	builtInFunctions                    vmcommon.BuiltInFunctionContainer
	gasConfig                           *vmcommon.GasCost
	shardCoordinator                    vmcommon.Coordinator
	epochNotifier                       vmcommon.EpochNotifier
	dctNFTImprovementV1ActivationEpoch uint32
}

// NewBuiltInFunctionsFactory creates a factory which will instantiate the built in functions contracts
func NewBuiltInFunctionsFactory(args ArgsCreateBuiltInFunctionContainer) (*builtInFuncFactory, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(args.Accounts) {
		return nil, ErrNilAccountsAdapter
	}
	if args.MapDNSAddresses == nil {
		return nil, ErrNilDnsAddresses
	}
	if check.IfNil(args.ShardCoordinator) {
		return nil, ErrNilShardCoordinator
	}
	if check.IfNil(args.EpochNotifier) {
		return nil, ErrNilEpochHandler
	}

	b := &builtInFuncFactory{
		mapDNSAddresses:                     args.MapDNSAddresses,
		enableUserNameChange:                args.EnableUserNameChange,
		marshalizer:                         args.Marshalizer,
		accounts:                            args.Accounts,
		shardCoordinator:                    args.ShardCoordinator,
		epochNotifier:                       args.EpochNotifier,
		dctNFTImprovementV1ActivationEpoch: args.DCTNFTImprovementV1ActivationEpoch,
	}

	var err error
	b.gasConfig, err = createGasConfig(args.GasMap)
	if err != nil {
		return nil, err
	}
	b.builtInFunctions = NewBuiltInFunctionContainer()

	return b, nil
}

// GasScheduleChange is called when gas schedule is changed, thus all contracts must be updated
func (b *builtInFuncFactory) GasScheduleChange(gasSchedule map[string]map[string]uint64) {
	newGasConfig, err := createGasConfig(gasSchedule)
	if err != nil {
		return
	}

	b.gasConfig = newGasConfig
	for key := range b.builtInFunctions.Keys() {
		builtInFunc, errGet := b.builtInFunctions.Get(key)
		if errGet != nil {
			return
		}

		builtInFunc.SetNewGasConfig(b.gasConfig)
	}
}

// CreateBuiltInFunctionContainer will create the list of built-in functions
func (b *builtInFuncFactory) CreateBuiltInFunctionContainer() (vmcommon.BuiltInFunctionContainer, error) {

	b.builtInFunctions = NewBuiltInFunctionContainer()
	var newFunc vmcommon.BuiltinFunction
	newFunc = NewClaimDeveloperRewardsFunc(b.gasConfig.BuiltInCost.ClaimDeveloperRewards)
	err := b.builtInFunctions.Add(vmcommon.BuiltInFunctionClaimDeveloperRewards, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc = NewChangeOwnerAddressFunc(b.gasConfig.BuiltInCost.ChangeOwnerAddress)
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionChangeOwnerAddress, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewSaveUserNameFunc(b.gasConfig.BuiltInCost.SaveUserName, b.mapDNSAddresses, b.enableUserNameChange)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionSetUserName, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewSaveKeyValueStorageFunc(b.gasConfig.BaseOperationCost, b.gasConfig.BuiltInCost.SaveKeyValue)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionSaveKeyValue, newFunc)
	if err != nil {
		return nil, err
	}

	pauseFunc, err := NewDCTPauseFunc(b.accounts, true)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTPause, pauseFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTTransferFunc(b.gasConfig.BuiltInCost.DCTTransfer, b.marshalizer, pauseFunc, b.shardCoordinator)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTBurnFunc(b.gasConfig.BuiltInCost.DCTBurn, b.marshalizer, pauseFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTBurn, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTFreezeWipeFunc(b.marshalizer, true, false)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTFreeze, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTFreezeWipeFunc(b.marshalizer, false, false)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTUnFreeze, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTFreezeWipeFunc(b.marshalizer, false, true)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTWipe, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTPauseFunc(b.accounts, false)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTUnPause, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTRolesFunc(b.marshalizer, false)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionUnSetDCTRole, newFunc)
	if err != nil {
		return nil, err
	}

	setRoleFunc, err := NewDCTRolesFunc(b.marshalizer, true)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionSetDCTRole, setRoleFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTLocalBurnFunc(b.gasConfig.BuiltInCost.DCTLocalBurn, b.marshalizer, pauseFunc, setRoleFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTLocalBurn, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTLocalMintFunc(b.gasConfig.BuiltInCost.DCTLocalMint, b.marshalizer, pauseFunc, setRoleFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTLocalMint, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTAddQuantityFunc(b.gasConfig.BuiltInCost.DCTNFTAddQuantity, b.marshalizer, pauseFunc, setRoleFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTNFTAddQuantity, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTBurnFunc(b.gasConfig.BuiltInCost.DCTNFTBurn, b.marshalizer, pauseFunc, setRoleFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTNFTBurn, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTCreateFunc(b.gasConfig.BuiltInCost.DCTNFTCreate, b.gasConfig.BaseOperationCost, b.marshalizer, pauseFunc, setRoleFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTNFTCreate, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTTransferFunc(b.gasConfig.BuiltInCost.DCTNFTTransfer, b.marshalizer, pauseFunc, b.accounts, b.shardCoordinator, b.gasConfig.BaseOperationCost)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTNFTTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTCreateRoleTransfer(b.marshalizer, b.accounts, b.shardCoordinator)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTNFTCreateRoleTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTUpdateAttributesFunc(b.gasConfig.BuiltInCost.DCTNFTUpdateAttributes, b.gasConfig.BaseOperationCost, b.marshalizer, pauseFunc, setRoleFunc, b.dctNFTImprovementV1ActivationEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTNFTUpdateAttributes, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTAddUriFunc(b.gasConfig.BuiltInCost.DCTNFTAddURI, b.gasConfig.BaseOperationCost, b.marshalizer, pauseFunc, setRoleFunc, b.dctNFTImprovementV1ActivationEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionDCTNFTAddURI, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTMultiTransferFunc(b.gasConfig.BuiltInCost.DCTNFTMultiTransfer, b.marshalizer, pauseFunc, b.accounts, b.shardCoordinator, b.gasConfig.BaseOperationCost, b.dctNFTImprovementV1ActivationEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(vmcommon.BuiltInFunctionMultiDCTNFTTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	return b.builtInFunctions, nil
}

func createGasConfig(gasMap map[string]map[string]uint64) (*vmcommon.GasCost, error) {
	baseOps := &vmcommon.BaseOperationCost{}
	err := mapstructure.Decode(gasMap[vmcommon.BaseOperationCostString], baseOps)
	if err != nil {
		return nil, err
	}

	err = check.ForZeroUintFields(*baseOps)
	if err != nil {
		return nil, err
	}

	builtInOps := &vmcommon.BuiltInCost{}
	err = mapstructure.Decode(gasMap[vmcommon.BuiltInCostString], builtInOps)
	if err != nil {
		return nil, err
	}

	err = check.ForZeroUintFields(*builtInOps)
	if err != nil {
		return nil, err
	}

	gasCost := vmcommon.GasCost{
		BaseOperationCost: *baseOps,
		BuiltInCost:       *builtInOps,
	}

	return &gasCost, nil
}

// SetPayableHandler sets the payable interface to the needed functions
func SetPayableHandler(container vmcommon.BuiltInFunctionContainer, payableHandler vmcommon.PayableHandler) error {
	listOfTransferFunc := []string{
		vmcommon.BuiltInFunctionMultiDCTNFTTransfer,
		vmcommon.BuiltInFunctionDCTNFTTransfer,
		vmcommon.BuiltInFunctionDCTTransfer}

	for _, transferFunc := range listOfTransferFunc {
		builtInFunc, err := container.Get(transferFunc)
		if err != nil {
			return err
		}

		dctTransferFunc, ok := builtInFunc.(vmcommon.AcceptPayableHandler)
		if !ok {
			return ErrWrongTypeAssertion
		}

		err = dctTransferFunc.SetPayableHandler(payableHandler)
		if err != nil {
			return err
		}
	}

	return nil
}

// IsInterfaceNil returns true if underlying object is nil
func (b *builtInFuncFactory) IsInterfaceNil() bool {
	return b == nil
}
