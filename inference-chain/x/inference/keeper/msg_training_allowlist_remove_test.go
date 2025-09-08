package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestMsgRemoveUserFromTrainingAllowList(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	wctx := sdk.UnwrapSDKContext(ctx)

	// unauthorized authority should fail
	_, err := ms.RemoveUserFromTrainingAllowList(wctx, &types.MsgRemoveUserFromTrainingAllowList{
		Authority: "invalid",
		Address:   "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2",
	})
	require.Error(t, err)

	addr := "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"
	acc, e := sdk.AccAddressFromBech32(addr)
	require.NoError(t, e)

	// pre-add to allow list
	err = k.TrainingAllowListSet.Set(wctx, acc)
	require.NoError(t, err)

	// remove with proper authority
	_, err = ms.RemoveUserFromTrainingAllowList(wctx, &types.MsgRemoveUserFromTrainingAllowList{
		Authority: k.GetAuthority(),
		Address:   addr,
	})
	require.NoError(t, err)

	ok, e := k.TrainingAllowListSet.Has(wctx, acc)
	require.NoError(t, e)
	require.False(t, ok)

	// idempotent: remove again should not error
	_, err = ms.RemoveUserFromTrainingAllowList(wctx, &types.MsgRemoveUserFromTrainingAllowList{
		Authority: k.GetAuthority(),
		Address:   addr,
	})
	require.NoError(t, err)
}
