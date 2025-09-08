package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

func TestSubmitTrainingKvRecord_AllowListEnforced(t *testing.T) {
	k, ctx := keepertest.InferenceKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	wctx := sdk.UnwrapSDKContext(ctx)

	creator := "gonka1hgt9lxxxwpsnc3yn2nheqqy9a8vlcjwvgzpve2"

	// not allowed
	_, err := ms.SubmitTrainingKvRecord(wctx, &types.MsgSubmitTrainingKvRecord{
		Creator: creator,
		TaskId:  1,
		Key:     "k",
		Value:   "v",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTrainingNotAllowed)

	// allow
	acc, e := sdk.AccAddressFromBech32(creator)
	require.NoError(t, e)
	require.NoError(t, k.TrainingAllowListSet.Set(wctx, acc))

	// should succeed now
	_, err = ms.SubmitTrainingKvRecord(wctx, &types.MsgSubmitTrainingKvRecord{
		Creator: creator,
		TaskId:  1,
		Key:     "k",
		Value:   "v",
	})
	require.NoError(t, err)
}
