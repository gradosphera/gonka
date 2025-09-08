package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestCreateDummyTrainingTask_AllowListEnforced(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	wctx := sdk.UnwrapSDKContext(ctx)

	creator := "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"

	// not on allow list -> expect ErrTrainingNotAllowed
	_, err := ms.CreateDummyTrainingTask(wctx, &types.MsgCreateDummyTrainingTask{
		Creator: creator,
		Task:    &types.TrainingTask{Id: 1},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTrainingNotAllowed)

	// add to allow list
	acc, e := sdk.AccAddressFromBech32(creator)
	require.NoError(t, e)
	require.NoError(t, k.TrainingAllowListSet.Set(wctx, acc))

	// now allowed -> should succeed
	resp, err := ms.CreateDummyTrainingTask(wctx, &types.MsgCreateDummyTrainingTask{
		Creator: creator,
		Task:    &types.TrainingTask{Id: 2},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Task)
}
