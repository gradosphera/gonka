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
	_, found := k.GetParticipant(ctx, msg.Creator)
	if !found {
		return nil, types.ErrParticipantNotFound
	}

	task, found := k.GetTrainingTask(ctx, msg.TaskId) // ensure task exists
	if !found {
		return nil, types.ErrTrainingTaskNotFound
	}

	creatorIsAssigned := false
	for _, assignee := range task.Assignees {
		if assignee.Participant == msg.Creator {
			creatorIsAssigned = true
			break
		}
	}

	if !creatorIsAssigned {
		return nil, types.ErrTrainingTaskAlreadyAssigned
	}

	record := types.TrainingTaskKVRecord{
		TaskId:      msg.TaskId,
		Participant: msg.Creator,
		Key:         msg.Key,
		Value:       msg.Value,
	}
	k.SetTrainingKVRecord(ctx, &record)

	return &types.MsgSubmitTrainingKvRecordResponse{}, nil
}
