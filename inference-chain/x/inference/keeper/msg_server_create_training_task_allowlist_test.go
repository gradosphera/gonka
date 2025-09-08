package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestCreateTrainingTask_AllowListEnforced(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	wctx := sdk.UnwrapSDKContext(ctx)

	// not on allow list -> expect ErrTrainingNotAllowed
	_, err := ms.CreateTrainingTask(wctx, &types.MsgCreateTrainingTask{
		Creator:           "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2",
		HardwareResources: []*types.TrainingHardwareResources{},
		Config:            &types.TrainingConfig{},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTrainingNotAllowed)

	// add to allow list
	acc, e := sdk.AccAddressFromBech32("gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2")
	require.NoError(t, e)
	require.NoError(t, k.TrainingAllowListSet.Set(wctx, acc))

	// now allowed -> should succeed
	resp, err := ms.CreateTrainingTask(wctx, &types.MsgCreateTrainingTask{
		Creator:           "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2",
		HardwareResources: []*types.TrainingHardwareResources{},
		Config:            &types.TrainingConfig{},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Task)
}
