package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestMsgAddUserToTrainingAllowList(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	wctx := sdk.UnwrapSDKContext(ctx)

	// unauthorized authority should fail
	_, err := ms.AddUserToTrainingAllowList(wctx, &types.MsgAddUserToTrainingAllowList{
		Authority: "invalid",
		Address:   "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2", // any bech32
	})
	require.Error(t, err)

	// valid authority should add address
	addr := "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"
	_, err = ms.AddUserToTrainingAllowList(wctx, &types.MsgAddUserToTrainingAllowList{
		Authority: k.GetAuthority(),
		Address:   addr,
	})
	require.NoError(t, err)

	acc, e := sdk.AccAddressFromBech32(addr)
	require.NoError(t, e)
	ok, e := k.TrainingAllowListSet.Has(wctx, acc)
	require.NoError(t, e)
	require.True(t, ok)
}
