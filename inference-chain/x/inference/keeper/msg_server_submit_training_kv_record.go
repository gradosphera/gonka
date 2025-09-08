package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/types"
)

func (k msgServer) SubmitTrainingKvRecord(goCtx context.Context, msg *types.MsgSubmitTrainingKvRecord) (*types.MsgSubmitTrainingKvRecordResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.CheckTrainingAllowList(ctx, msg); err != nil {
		return nil, err
	}

	// TODO: check participant and training task exists?
	record := types.TrainingTaskKVRecord{
		TaskId:      msg.TaskId,
		Participant: msg.Creator,
		Key:         msg.Key,
		Value:       msg.Value,
	}
	k.SetTrainingKVRecord(ctx, &record)

	return &types.MsgSubmitTrainingKvRecordResponse{}, nil
}
