package keeper

import (
	"context"
	"reflect"

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
	switch req.Role {
	case 0:
		if err := k.TrainingExecAllowListSet.Walk(ctx, nil, func(a sdk.AccAddress) (bool, error) {
			addrs = append(addrs, a.String())
			return false, nil
		}); err != nil {
			return nil, err
		}
	case 1:
		if err := k.TrainingStartAllowListSet.Walk(ctx, nil, func(a sdk.AccAddress) (bool, error) {
			addrs = append(addrs, a.String())
			return false, nil
		}); err != nil {
			return nil, err
		}

	}
	return &types.QueryTrainingAllowListResponse{Addresses: addrs}, nil
}

var execMessages = map[reflect.Type]struct{}{
	reflect.TypeOf((*types.MsgSubmitTrainingKvRecord)(nil)): {},
	reflect.TypeOf((*types.MsgJoinTraining)(nil)):           {},
	reflect.TypeOf((*types.MsgJoinTrainingStatus)(nil)):     {},
	reflect.TypeOf((*types.MsgSetBarrier)(nil)):             {},
	reflect.TypeOf((*types.MsgTrainingHeartbeat)(nil)):      {},
}

var startMessages = map[reflect.Type]struct{}{
	reflect.TypeOf((*types.MsgAssignTrainingTask)(nil)):             {},
	reflect.TypeOf((*types.MsgClaimTrainingTaskForAssignment)(nil)): {},
	reflect.TypeOf((*types.MsgCreateDummyTrainingTask)(nil)):        {},
	reflect.TypeOf((*types.MsgCreateTrainingTask)(nil)):             {},
}

func (k msgServer) CheckTrainingAllowList(ctx context.Context, msg HasCreator) error {
	creator, err := sdk.AccAddressFromBech32(msg.GetCreator())
	if err != nil {
		return err
	}
	var allowed = true
	if _, ok := execMessages[reflect.TypeOf(msg)]; ok {
		allowed, err = k.TrainingExecAllowListSet.Has(ctx, creator)
	}
	if _, ok := startMessages[reflect.TypeOf(msg)]; ok {
		onStart, err2 := k.TrainingStartAllowListSet.Has(ctx, creator)
		if err2 != nil {
			return err2
		}
		// Not current, but in case we add BOTH requirements in the future
		allowed = onStart && allowed
	}
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
