package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k Keeper) MLNodeVersion(goCtx context.Context, req *types.QueryGetMLNodeVersionRequest) (*types.QueryGetMLNodeVersionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	val, found := k.GetMLNodeVersion(ctx)
	if !found {
		k.LogWarn("MLNode version not found", types.Nodes)
	}

	return &types.QueryGetMLNodeVersionResponse{MlnodeVersion: val}, nil
}
