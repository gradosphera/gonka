package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgRemoveUserFromTrainingAllowList{}

func NewMsgRemoveUserFromTrainingAllowList(creator string, authority string, address string) *MsgRemoveUserFromTrainingAllowList {
	return &MsgRemoveUserFromTrainingAllowList{
		Authority: authority,
		Address:   address,
	}
}

func (msg *MsgRemoveUserFromTrainingAllowList) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	_, err = sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address to remove (%s)", err)
	}
	return nil
}
