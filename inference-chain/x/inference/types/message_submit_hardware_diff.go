package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgSubmitHardwareDiff{}

func NewMsgSubmitHardwareDiff(creator string) *MsgSubmitHardwareDiff {
	return &MsgSubmitHardwareDiff{
		Creator: creator,
	}
}

func (msg *MsgSubmitHardwareDiff) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	if len(msg.Removed) > MaxRemoved {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "removed has more than %d elements", MaxRemoved)
	}
	if len(msg.NewOrModified) > MaxNewOrModified {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "newOrModified has more than %d elements", MaxNewOrModified)
	}
	return nil
}

const MaxRemoved = 1000
const MaxNewOrModified = 1000
