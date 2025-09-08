package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgSetTrainingAllowList{}

func NewMsgSetTrainingAllowList(creator string, authority string, addresses []string) *MsgSetTrainingAllowList {
	return &MsgSetTrainingAllowList{
		Authority: authority,
		Addresses: addresses,
	}
}

func (msg *MsgSetTrainingAllowList) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	for _, address := range msg.Addresses {
		_, err := sdk.AccAddressFromBech32(address)
		if err != nil {
			return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address to set (%s)", err)
		}
	}
	return nil
}
