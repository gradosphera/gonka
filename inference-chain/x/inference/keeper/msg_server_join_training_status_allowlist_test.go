package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestJoinTrainingStatus_AllowListEnforced(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	wctx := sdk.UnwrapSDKContext(ctx)

	creator := "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"
	req := &types.JoinTrainingRequest{NodeId: creator + "/node1", RunId: 1, OuterStep: 0}

	// not allowed
	_, err := ms.JoinTrainingStatus(wctx, &types.MsgJoinTrainingStatus{Creator: creator, Req: req})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTrainingNotAllowed)

	// allow
	acc, e := sdk.AccAddressFromBech32(creator)
	require.NoError(t, e)
	require.NoError(t, k.TrainingExecAllowListSet.Set(wctx, acc))

	// should not fail with ErrTrainingNotAllowed
	_, err = ms.JoinTrainingStatus(wctx, &types.MsgJoinTrainingStatus{Creator: creator, Req: req})
	if err != nil {
		require.NotErrorIs(t, err, types.ErrTrainingNotAllowed)
	}
}
