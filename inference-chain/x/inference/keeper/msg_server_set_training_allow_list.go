package keeper

import (
	"context"

	"cosmossdk.io/collections"
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

	var set collections.KeySet[sdk.AccAddress]
	switch msg.Role {
	case types.TrainingRole_ROLE_EXEC:
		set = k.TrainingExecAllowListSet
	case types.TrainingRole_ROLE_START:
		set = k.TrainingStartAllowListSet
	default:
		return nil, types.ErrUnknownAllowList
	}
	response, err := k.setAllowList(ctx, msg, set)
	if err != nil {
		return response, err
	}

	return &types.MsgSetTrainingAllowListResponse{}, nil
}

func (k msgServer) setAllowList(ctx sdk.Context, msg *types.MsgSetTrainingAllowList, set collections.KeySet[sdk.AccAddress]) (*types.MsgSetTrainingAllowListResponse, error) {
	if err := set.Clear(ctx, nil); err != nil {
		return nil, err
	}
	k.LogInfo("Cleared training allow list", types.Training)

	for _, a := range msg.Addresses {
		addr, err := sdk.AccAddressFromBech32(a)
		if err != nil {
			return nil, err
		}
		if err := set.Set(ctx, addr); err != nil {
			return nil, err
		}
		k.LogInfo("Added user to training allow list", types.Training, "address", addr)
	}
	return nil, nil
}
