package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/types"
)

func (k msgServer) SetTrainingAllowList(goCtx context.Context, msg *types.MsgSetTrainingAllowList) (*types.MsgSetTrainingAllowListResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", k.GetAuthority(), msg.Authority)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate addresses
	for _, a := range msg.Addresses {
		if _, err := sdk.AccAddressFromBech32(a); err != nil {
			return nil, err
		}
	}

	if err := k.TrainingAllowListSet.Clear(ctx, nil); err != nil {
		return nil, err
	}

	for _, a := range msg.Addresses {
		addr, err := sdk.AccAddressFromBech32(a)
		if err != nil {
			return nil, err
		}
		if err := k.TrainingAllowListSet.Set(ctx, addr); err != nil {
			return nil, err
		}
	}

	return &types.MsgSetTrainingAllowListResponse{}, nil
}
