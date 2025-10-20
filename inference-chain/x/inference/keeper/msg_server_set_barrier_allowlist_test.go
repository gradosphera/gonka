package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestSetBarrier_AllowListEnforced(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	wctx := sdk.UnwrapSDKContext(ctx)

	creator := "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"

	req := &types.SetBarrierRequest{NodeId: creator + "/node1", RunId: 1, OuterStep: 0, BarrierId: "b1"}

	// not allowed
	_, err := ms.SetBarrier(wctx, &types.MsgSetBarrier{Creator: creator, Req: req})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTrainingNotAllowed)

	// allow
	acc, e := sdk.AccAddressFromBech32(creator)
	require.NoError(t, e)
	require.NoError(t, k.TrainingExecAllowListSet.Set(wctx, acc))

	// now do not fail with ErrTrainingNotAllowed (may still fail for other reasons in future changes)
	_, err = ms.SetBarrier(wctx, &types.MsgSetBarrier{Creator: creator, Req: req})
	if err != nil {
		require.NotErrorIs(t, err, types.ErrTrainingNotAllowed)
	}
}
