package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k Keeper) TrainingAllowList(goCtx context.Context, req *types.QueryTrainingAllowListRequest) (*types.QueryTrainingAllowListResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Collect all addresses from the allow list
	var addrs []string
	if err := k.TrainingAllowListSet.Walk(ctx, nil, func(a sdk.AccAddress) (bool, error) {
		addrs = append(addrs, a.String())
		return false, nil
	}); err != nil {
		return nil, err
	}

	return &types.QueryTrainingAllowListResponse{Addresses: addrs}, nil
}

func (k msgServer) CheckTrainingAllowList(ctx context.Context, msg HasCreator) error {
	creator, err := sdk.AccAddressFromBech32(msg.GetCreator())
	if err != nil {
		return err
	}
	allowed, err := k.TrainingAllowListSet.Has(ctx, creator)
	if err != nil {
		return err
	}
	if !allowed {
		return types.ErrTrainingNotAllowed
	}
	return nil
}

type HasCreator interface {
	GetCreator() string
}
