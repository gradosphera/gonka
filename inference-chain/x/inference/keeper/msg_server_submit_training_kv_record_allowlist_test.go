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

	err := k.Participants.Set(wctx, sdk.MustAccAddressFromBech32(creator), types.Participant{
		Index:   creator,
		Address: creator,
	})
	require.NoError(t, err)

	k.SetTrainingTask(ctx, &types.TrainingTask{
		Id: 1,
		Assignees: []*types.TrainingTaskAssignee{
			{
				Participant: creator,
			},
		},
	})
	// not allowed
	_, err = ms.SubmitTrainingKvRecord(wctx, &types.MsgSubmitTrainingKvRecord{
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
	require.NoError(t, k.TrainingExecAllowListSet.Set(wctx, acc))

	// should succeed now
	_, err = ms.SubmitTrainingKvRecord(wctx, &types.MsgSubmitTrainingKvRecord{
		Creator: creator,
		TaskId:  1,
		Key:     "k",
		Value:   "v",
	})
	require.NoError(t, err)
}
