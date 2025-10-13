package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestMsgSetTrainingAllowList(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	wctx := sdk.UnwrapSDKContext(ctx)

	// unauthorized should fail
	_, err := ms.SetTrainingAllowList(wctx, &types.MsgSetTrainingAllowList{
		Authority: "invalid",
		Addresses: []string{"gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"},
	})
	require.Error(t, err)

	// valid: set with duplicates and unsorted
	a1 := "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"
	a2 := "gonka1q6hag67dl53wl99vzg42z8eyzfz2xlkvqkpz3w"
	a3 := "gonka1ry2uuyhslzyg4daxck5tg3y8yf68k3fzrjl40e"

	_, err = ms.SetTrainingAllowList(wctx, &types.MsgSetTrainingAllowList{
		Authority: k.GetAuthority(),
		Addresses: []string{a2, a1, a2, a3},
		Role:      types.TrainingRole_ROLE_EXEC,
	})
	require.NoError(t, err)

	// verify store contents equals {a1, a2, a3}
	acc1, e := sdk.AccAddressFromBech32(a1)
	require.NoError(t, e)
	acc2, e := sdk.AccAddressFromBech32(a2)
	require.NoError(t, e)
	acc3, e := sdk.AccAddressFromBech32(a3)
	require.NoError(t, e)

	ok, e := k.TrainingExecAllowListSet.Has(wctx, acc1)
	require.NoError(t, e)
	require.True(t, ok)
	ok, e = k.TrainingExecAllowListSet.Has(wctx, acc2)
	require.NoError(t, e)
	require.True(t, ok)
	ok, e = k.TrainingExecAllowListSet.Has(wctx, acc3)
	require.NoError(t, e)
	require.True(t, ok)

	// unauthorized should fail for ROLE_START
	_, err = ms.SetTrainingAllowList(wctx, &types.MsgSetTrainingAllowList{
		Authority: "invalid",
		Addresses: []string{"gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"},
		Role:      types.TrainingRole_ROLE_START,
	})
	require.Error(t, err)

	// valid: set with duplicates and unsorted for ROLE_START
	_, err = ms.SetTrainingAllowList(wctx, &types.MsgSetTrainingAllowList{
		Authority: k.GetAuthority(),
		Addresses: []string{a2, a1, a2, a3},
		Role:      types.TrainingRole_ROLE_START,
	})
	require.NoError(t, err)

	// verify store contents equals {a1, a2, a3} for ROLE_START
	ok, e = k.TrainingStartAllowListSet.Has(wctx, acc1)
	require.NoError(t, e)
	require.True(t, ok)
	ok, e = k.TrainingStartAllowListSet.Has(wctx, acc2)
	require.NoError(t, e)
	require.True(t, ok)
	ok, e = k.TrainingStartAllowListSet.Has(wctx, acc3)
	require.NoError(t, e)
	require.True(t, ok)

}
