package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgAddUserToTrainingAllowList{}

func NewMsgAddUserToTrainingAllowList(creator string, authority string, address string) *MsgAddUserToTrainingAllowList {
	return &MsgAddUserToTrainingAllowList{
		Authority: authority,
		Address:   address,
	}
}

func (msg *MsgAddUserToTrainingAllowList) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	_, err = sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address to add (%s)", err)
	}
	return nil
}
