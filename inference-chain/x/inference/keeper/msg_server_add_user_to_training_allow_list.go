package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/types"
)

func (k msgServer) AddUserToTrainingAllowList(goCtx context.Context, msg *types.MsgAddUserToTrainingAllowList) (*types.MsgAddUserToTrainingAllowListResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", k.GetAuthority(), msg.Authority)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	addr, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return nil, err
	}
	switch msg.Role {
	case types.TrainingRole_ROLE_EXEC:
		if err := k.TrainingExecAllowListSet.Set(ctx, addr); err != nil {
			return nil, err
		}
	case types.TrainingRole_ROLE_START:
		if err := k.TrainingStartAllowListSet.Set(ctx, addr); err != nil {
			return nil, err
		}
	}

	k.LogInfo("Added user to training allow list", types.Training, "address", addr, "role", msg.Role)

	return &types.MsgAddUserToTrainingAllowListResponse{}, nil
}
