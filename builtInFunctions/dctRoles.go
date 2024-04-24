package builtInFunctions

import (
	"bytes"

	"github.com/numbatx/gn-vm-common"
	"github.com/numbatx/gn-vm-common/check"
	"github.com/numbatx/gn-vm-common/data/dct"
)

var roleKeyPrefix = []byte(vmcommon.NumbatProtectedKeyPrefix + vmcommon.DCTRoleIdentifier + vmcommon.DCTKeyIdentifier)

type dctRoles struct {
	baseAlwaysActive
	set         bool
	marshalizer vmcommon.Marshalizer
}

// NewDCTRolesFunc returns the dct change roles built-in function component
func NewDCTRolesFunc(
	marshalizer vmcommon.Marshalizer,
	set bool,
) (*dctRoles, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}

	e := &dctRoles{
		set:         set,
		marshalizer: marshalizer,
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *dctRoles) SetNewGasConfig(_ *vmcommon.GasCost) {
}

// ProcessBuiltinFunction resolves DCT change roles function call
func (e *dctRoles) ProcessBuiltinFunction(
	_, acntDst vmcommon.UserAccountHandler,
	vmInput *vmcommon.ContractCallInput,
) (*vmcommon.VMOutput, error) {
	err := checkBasicDCTArguments(vmInput)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(vmInput.CallerAddr, vmcommon.DCTSCAddress) {
		return nil, ErrAddressIsNotDCTSystemSC
	}
	if check.IfNil(acntDst) {
		return nil, ErrNilUserAccount
	}

	dctTokenRoleKey := append(roleKeyPrefix, vmInput.Arguments[0]...)

	roles, _, err := getDCTRolesForAcnt(e.marshalizer, acntDst, dctTokenRoleKey)
	if err != nil {
		return nil, err
	}

	if e.set {
		roles.Roles = append(roles.Roles, vmInput.Arguments[1:]...)
	} else {
		deleteRoles(roles, vmInput.Arguments[1:])
	}

	err = saveRolesToAccount(acntDst, dctTokenRoleKey, roles, e.marshalizer)
	if err != nil {
		return nil, err
	}

	vmOutput := &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}
	return vmOutput, nil
}

func deleteRoles(roles *dct.DCTRoles, deleteRoles [][]byte) {
	for _, deleteRole := range deleteRoles {
		index, exist := doesRoleExist(roles, deleteRole)
		if !exist {
			continue
		}

		copy(roles.Roles[index:], roles.Roles[index+1:])
		roles.Roles[len(roles.Roles)-1] = nil
		roles.Roles = roles.Roles[:len(roles.Roles)-1]
	}
}

func doesRoleExist(roles *dct.DCTRoles, role []byte) (int, bool) {
	for i, currentRole := range roles.Roles {
		if bytes.Equal(currentRole, role) {
			return i, true
		}
	}
	return -1, false
}

func getDCTRolesForAcnt(
	marshalizer vmcommon.Marshalizer,
	acnt vmcommon.UserAccountHandler,
	key []byte,
) (*dct.DCTRoles, bool, error) {
	roles := &dct.DCTRoles{
		Roles: make([][]byte, 0),
	}

	marshaledData, err := acnt.AccountDataHandler().RetrieveValue(key)
	if err != nil || len(marshaledData) == 0 {
		return roles, true, nil
	}

	err = marshalizer.Unmarshal(roles, marshaledData)
	if err != nil {
		return nil, false, err
	}

	return roles, false, nil
}

// CheckAllowedToExecute returns error if the account is not allowed to execute the given action
func (e *dctRoles) CheckAllowedToExecute(account vmcommon.UserAccountHandler, tokenID []byte, action []byte) error {
	if check.IfNil(account) {
		return ErrNilUserAccount
	}

	dctTokenRoleKey := append(roleKeyPrefix, tokenID...)
	roles, isNew, err := getDCTRolesForAcnt(e.marshalizer, account, dctTokenRoleKey)
	if err != nil {
		return err
	}
	if isNew {
		return ErrActionNotAllowed
	}

	for _, role := range roles.Roles {
		if bytes.Equal(role, action) {
			return nil
		}
	}

	return ErrActionNotAllowed
}

// IsInterfaceNil returns true if underlying object in nil
func (e *dctRoles) IsInterfaceNil() bool {
	return e == nil
}
