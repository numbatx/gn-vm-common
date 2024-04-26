package builtInFunctions

import (
	"github.com/numbatx/gn-core/core"
	"github.com/numbatx/gn-core/core/check"
	vmcommon "github.com/numbatx/gn-vm-common"
	"github.com/mitchellh/mapstructure"
)

// ArgsCreateBuiltInFunctionContainer defines the input arguments to create built in functions container
type ArgsCreateBuiltInFunctionContainer struct {
	GasMap                              map[string]map[string]uint64
	MapDNSAddresses                     map[string]struct{}
	EnableUserNameChange                bool
	Marshalizer                         vmcommon.Marshalizer
	Accounts                            vmcommon.AccountsAdapter
	ShardCoordinator                    vmcommon.Coordinator
	EpochNotifier                       vmcommon.EpochNotifier
	DCTNFTImprovementV1ActivationEpoch uint32
	DCTTransferRoleEnableEpoch         uint32
	GlobalMintBurnDisableEpoch          uint32
	DCTTransferToMetaEnableEpoch       uint32
	NFTCreateMultiShardEnableEpoch      uint32
	SaveNFTToSystemAccountEnableEpoch   uint32
}

type builtInFuncCreator struct {
	mapDNSAddresses                     map[string]struct{}
	enableUserNameChange                bool
	marshalizer                         vmcommon.Marshalizer
	accounts                            vmcommon.AccountsAdapter
	builtInFunctions                    vmcommon.BuiltInFunctionContainer
	gasConfig                           *vmcommon.GasCost
	shardCoordinator                    vmcommon.Coordinator
	epochNotifier                       vmcommon.EpochNotifier
	dctStorageHandler                  vmcommon.DCTNFTStorageHandler
	dctNFTImprovementV1ActivationEpoch uint32
	dctTransferRoleEnableEpoch         uint32
	globalMintBurnDisableEpoch          uint32
	dctTransferToMetaEnableEpoch       uint32
	nftCreateMultiShardEnableEpoch      uint32
	saveNFTToSystemAccountEnableEpoch   uint32
}

// NewBuiltInFunctionsCreator creates a component which will instantiate the built in functions contracts
func NewBuiltInFunctionsCreator(args ArgsCreateBuiltInFunctionContainer) (*builtInFuncCreator, error) {
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

	b := &builtInFuncCreator{
		mapDNSAddresses:                     args.MapDNSAddresses,
		enableUserNameChange:                args.EnableUserNameChange,
		marshalizer:                         args.Marshalizer,
		accounts:                            args.Accounts,
		shardCoordinator:                    args.ShardCoordinator,
		epochNotifier:                       args.EpochNotifier,
		dctNFTImprovementV1ActivationEpoch: args.DCTNFTImprovementV1ActivationEpoch,
		dctTransferRoleEnableEpoch:         args.DCTTransferRoleEnableEpoch,
		globalMintBurnDisableEpoch:          args.GlobalMintBurnDisableEpoch,
		dctTransferToMetaEnableEpoch:       args.DCTTransferToMetaEnableEpoch,
		nftCreateMultiShardEnableEpoch:      args.NFTCreateMultiShardEnableEpoch,
		saveNFTToSystemAccountEnableEpoch:   args.SaveNFTToSystemAccountEnableEpoch,
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
func (b *builtInFuncCreator) GasScheduleChange(gasSchedule map[string]map[string]uint64) {
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

// NFTStorageHandler will return the dct storage handler from the built in functions factory
func (b *builtInFuncCreator) NFTStorageHandler() vmcommon.SimpleDCTNFTStorageHandler {
	return b.dctStorageHandler
}

// CreateBuiltInFunctionContainer will create the list of built-in functions
func (b *builtInFuncCreator) CreateBuiltInFunctionContainer() (vmcommon.BuiltInFunctionContainer, error) {

	b.builtInFunctions = NewBuiltInFunctionContainer()
	var newFunc vmcommon.BuiltinFunction
	newFunc = NewClaimDeveloperRewardsFunc(b.gasConfig.BuiltInCost.ClaimDeveloperRewards)
	err := b.builtInFunctions.Add(core.BuiltInFunctionClaimDeveloperRewards, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc = NewChangeOwnerAddressFunc(b.gasConfig.BuiltInCost.ChangeOwnerAddress)
	err = b.builtInFunctions.Add(core.BuiltInFunctionChangeOwnerAddress, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewSaveUserNameFunc(b.gasConfig.BuiltInCost.SaveUserName, b.mapDNSAddresses, b.enableUserNameChange)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionSetUserName, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewSaveKeyValueStorageFunc(b.gasConfig.BaseOperationCost, b.gasConfig.BuiltInCost.SaveKeyValue)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionSaveKeyValue, newFunc)
	if err != nil {
		return nil, err
	}

	globalSettingsFunc, err := NewDCTGlobalSettingsFunc(b.accounts, true, core.BuiltInFunctionDCTPause, 0, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTPause, globalSettingsFunc)
	if err != nil {
		return nil, err
	}

	setRoleFunc, err := NewDCTRolesFunc(b.marshalizer, true)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionSetDCTRole, setRoleFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTTransferFunc(b.gasConfig.BuiltInCost.DCTTransfer, b.marshalizer, globalSettingsFunc, b.shardCoordinator, setRoleFunc, b.dctTransferToMetaEnableEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTBurnFunc(b.gasConfig.BuiltInCost.DCTBurn, b.marshalizer, globalSettingsFunc, b.globalMintBurnDisableEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTBurn, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTFreezeWipeFunc(b.marshalizer, true, false)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTFreeze, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTFreezeWipeFunc(b.marshalizer, false, false)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTUnFreeze, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTFreezeWipeFunc(b.marshalizer, false, true)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTWipe, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTGlobalSettingsFunc(b.accounts, false, core.BuiltInFunctionDCTUnPause, 0, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTUnPause, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTRolesFunc(b.marshalizer, false)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionUnSetDCTRole, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTLocalBurnFunc(b.gasConfig.BuiltInCost.DCTLocalBurn, b.marshalizer, globalSettingsFunc, setRoleFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTLocalBurn, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTLocalMintFunc(b.gasConfig.BuiltInCost.DCTLocalMint, b.marshalizer, globalSettingsFunc, setRoleFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTLocalMint, newFunc)
	if err != nil {
		return nil, err
	}

	args := ArgsNewDCTDataStorage{
		Accounts:                b.accounts,
		GlobalSettingsHandler:   globalSettingsFunc,
		Marshalizer:             b.marshalizer,
		SaveToSystemEnableEpoch: b.saveNFTToSystemAccountEnableEpoch,
		EpochNotifier:           b.epochNotifier,
		ShardCoordinator:        b.shardCoordinator,
	}
	b.dctStorageHandler, err = NewDCTDataStorage(args)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTAddQuantityFunc(b.gasConfig.BuiltInCost.DCTNFTAddQuantity, b.dctStorageHandler, globalSettingsFunc, setRoleFunc, b.saveNFTToSystemAccountEnableEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTNFTAddQuantity, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTBurnFunc(b.gasConfig.BuiltInCost.DCTNFTBurn, b.dctStorageHandler, globalSettingsFunc, setRoleFunc)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTNFTBurn, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTCreateFunc(b.gasConfig.BuiltInCost.DCTNFTCreate, b.gasConfig.BaseOperationCost, b.marshalizer, globalSettingsFunc, setRoleFunc, b.dctStorageHandler, b.accounts, b.saveNFTToSystemAccountEnableEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTNFTCreate, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTTransferFunc(b.gasConfig.BuiltInCost.DCTNFTTransfer, b.marshalizer, globalSettingsFunc, b.accounts, b.shardCoordinator, b.gasConfig.BaseOperationCost, setRoleFunc, b.dctTransferToMetaEnableEpoch, b.saveNFTToSystemAccountEnableEpoch, b.dctStorageHandler, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTNFTTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTCreateRoleTransfer(b.marshalizer, b.accounts, b.shardCoordinator)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTNFTCreateRoleTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTUpdateAttributesFunc(b.gasConfig.BuiltInCost.DCTNFTUpdateAttributes, b.gasConfig.BaseOperationCost, b.dctStorageHandler, globalSettingsFunc, setRoleFunc, b.dctNFTImprovementV1ActivationEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTNFTUpdateAttributes, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTAddUriFunc(b.gasConfig.BuiltInCost.DCTNFTAddURI, b.gasConfig.BaseOperationCost, b.dctStorageHandler, globalSettingsFunc, setRoleFunc, b.dctNFTImprovementV1ActivationEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTNFTAddURI, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTNFTMultiTransferFunc(b.gasConfig.BuiltInCost.DCTNFTMultiTransfer, b.marshalizer, globalSettingsFunc, b.accounts, b.shardCoordinator, b.gasConfig.BaseOperationCost, b.dctNFTImprovementV1ActivationEpoch, b.epochNotifier, setRoleFunc, b.dctTransferToMetaEnableEpoch, b.dctStorageHandler)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionMultiDCTNFTTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTGlobalSettingsFunc(b.accounts, true, core.BuiltInFunctionDCTSetLimitedTransfer, b.dctTransferRoleEnableEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTSetLimitedTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	newFunc, err = NewDCTGlobalSettingsFunc(b.accounts, false, core.BuiltInFunctionDCTUnSetLimitedTransfer, b.dctTransferRoleEnableEpoch, b.epochNotifier)
	if err != nil {
		return nil, err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDCTUnSetLimitedTransfer, newFunc)
	if err != nil {
		return nil, err
	}

	return b.builtInFunctions, nil
}

func createGasConfig(gasMap map[string]map[string]uint64) (*vmcommon.GasCost, error) {
	baseOps := &vmcommon.BaseOperationCost{}
	err := mapstructure.Decode(gasMap[core.BaseOperationCostString], baseOps)
	if err != nil {
		return nil, err
	}

	err = check.ForZeroUintFields(*baseOps)
	if err != nil {
		return nil, err
	}

	builtInOps := &vmcommon.BuiltInCost{}
	err = mapstructure.Decode(gasMap[core.BuiltInCostString], builtInOps)
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
		core.BuiltInFunctionMultiDCTNFTTransfer,
		core.BuiltInFunctionDCTNFTTransfer,
		core.BuiltInFunctionDCTTransfer}

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
func (b *builtInFuncCreator) IsInterfaceNil() bool {
	return b == nil
}
