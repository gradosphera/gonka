package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestQueryTrainingAllowList(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	wctx := sdk.UnwrapSDKContext(ctx)

	// empty returns empty list
	resp, err := k.TrainingAllowList(wctx, &types.QueryTrainingAllowListRequest{
		Role: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.Addresses)

	// add some addresses directly to the store
	a1 := "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"
	a2 := "gonka1q6hag67dl53wl99vzg42z8eyzfz2xlkvqkpz3w"
	a3 := "gonka1ry2uuyhslzyg4daxck5tg3y8yf68k3fzrjl40e"

	acc1, e := sdk.AccAddressFromBech32(a1)
	require.NoError(t, e)
	acc2, e := sdk.AccAddressFromBech32(a2)
	require.NoError(t, e)
	acc3, e := sdk.AccAddressFromBech32(a3)
	require.NoError(t, e)

	require.NoError(t, k.TrainingExecAllowListSet.Set(wctx, acc3))
	require.NoError(t, k.TrainingExecAllowListSet.Set(wctx, acc1))
	require.NoError(t, k.TrainingExecAllowListSet.Set(wctx, acc2))

	// query again; expect sorted lexicographically
	resp, err = k.TrainingAllowList(wctx, &types.QueryTrainingAllowListRequest{
		Role: 0,
	})
	require.NoError(t, err)
	require.Equal(t, []string{a2, a3, a1}, resp.Addresses)

	// empty returns empty list
	resp, err = k.TrainingAllowList(wctx, &types.QueryTrainingAllowListRequest{
		Role: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.Addresses)

	require.NoError(t, k.TrainingStartAllowListSet.Set(wctx, acc3))
	require.NoError(t, k.TrainingStartAllowListSet.Set(wctx, acc1))
	require.NoError(t, k.TrainingStartAllowListSet.Set(wctx, acc2))

	// query again; expect sorted lexicographically
	resp, err = k.TrainingAllowList(wctx, &types.QueryTrainingAllowListRequest{
		Role: 1,
	})
	require.NoError(t, err)
	require.Equal(t, []string{a2, a3, a1}, resp.Addresses)
}
