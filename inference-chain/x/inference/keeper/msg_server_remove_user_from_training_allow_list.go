package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/types"
)

func (k msgServer) RemoveUserFromTrainingAllowList(goCtx context.Context, msg *types.MsgRemoveUserFromTrainingAllowList) (*types.MsgRemoveUserFromTrainingAllowListResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", k.GetAuthority(), msg.Authority)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	addr, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return nil, err
	}
	if err := k.TrainingAllowListSet.Remove(ctx, addr); err != nil {
		return nil, err
	}

	return &types.MsgRemoveUserFromTrainingAllowListResponse{}, nil
}
