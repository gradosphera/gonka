package keeper

import (
	"context"

	sdkerrors "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/calculations"
	"github.com/productscience/inference/x/inference/types"
)

func (k msgServer) FinishInference(goCtx context.Context, msg *types.MsgFinishInference) (*types.MsgFinishInferenceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	k.LogInfo("FinishInference", types.Inferences, "inference_id", msg.InferenceId, "executed_by", msg.ExecutedBy, "created_by", msg.Creator)

	executor, found := k.GetParticipant(ctx, msg.ExecutedBy)
	if !found {
		k.LogError("FinishInference: executor not found", types.Inferences, "executed_by", msg.ExecutedBy)
		return nil, sdkerrors.Wrap(types.ErrParticipantNotFound, msg.ExecutedBy)
	}

	requestor, found := k.GetParticipant(ctx, msg.RequestedBy)
	if !found {
		k.LogError("FinishInference: requestor not found", types.Inferences, "requested_by", msg.RequestedBy)
		return nil, sdkerrors.Wrap(types.ErrParticipantNotFound, msg.RequestedBy)
	}

	transferAgent, found := k.GetParticipant(ctx, msg.TransferredBy)
	if !found {
		k.LogError("FinishInference: transfer agent not found", types.Inferences, "transferred_by", msg.TransferredBy)
		return nil, sdkerrors.Wrap(types.ErrParticipantNotFound, msg.TransferredBy)
	}

	err := k.verifyFinishKeys(ctx, msg, &transferAgent, &requestor, &executor)
	if err != nil {
		k.LogError("FinishInference: verifyKeys failed", types.Inferences, "error", err)
		return nil, sdkerrors.Wrap(types.ErrInvalidSignature, err.Error())
	}

	existingInference, found := k.GetInference(ctx, msg.InferenceId)

	if found && existingInference.FinishedProcessed() {
		k.LogError("FinishInference: inference already finished", types.Inferences, "inferenceId", msg.InferenceId)
		return nil, sdkerrors.Wrap(types.ErrInferenceFinishProcessed, "inference has already finished processed")
	}

	if found && existingInference.Status == types.InferenceStatus_EXPIRED {
		k.LogWarn("FinishInference: cannot finish expired inference", types.Inferences,
			"inferenceId", msg.InferenceId,
			"currentStatus", existingInference.Status,
			"executedBy", msg.ExecutedBy)
		return nil, sdkerrors.Wrap(types.ErrInferenceExpired, "inference has already expired")
	}

	// Record the current price only if this is the first message (StartInference not processed yet)
	// This ensures consistent pricing regardless of message arrival order
	if !existingInference.StartProcessed() {
		existingInference.Model = msg.Model
		k.RecordInferencePrice(goCtx, &existingInference, msg.InferenceId)
	} else if existingInference.Model == "" {
		k.LogError("FinishInference: model not set by the processed start message", types.Inferences,
			"inferenceId", msg.InferenceId,
			"executedBy", msg.ExecutedBy)
	} else if existingInference.Model != msg.Model {
		k.LogError("FinishInference: model mismatch", types.Inferences,
			"inferenceId", msg.InferenceId,
			"existingInference.Model", existingInference.Model,
			"msg.Model", msg.Model)
	}

	blockContext := calculations.BlockContext{
		BlockHeight:    ctx.BlockHeight(),
		BlockTimestamp: ctx.BlockTime().UnixMilli(),
	}

	inference, payments := calculations.ProcessFinishInference(&existingInference, msg, blockContext, k)

	finalInference, err := k.processInferencePayments(ctx, inference, payments)
	if err != nil {
		return nil, err
	}
	err = k.SetInference(ctx, *finalInference)
	if err != nil {
		return nil, err
	}
	if existingInference.IsCompleted() {
		err := k.handleInferenceCompleted(ctx, finalInference)
		if err != nil {
			return nil, err
		}
	}

	return &types.MsgFinishInferenceResponse{}, nil
}

func (k msgServer) verifyFinishKeys(ctx sdk.Context, msg *types.MsgFinishInference, transferAgent *types.Participant, requestor *types.Participant, executor *types.Participant) error {
	components := getFinishSignatureComponents(msg)
	// The extra seconds here need to be high enough to account for a very long inference.
	// Remember, deduping (via inferenceId) is our first defense against replay attacks, this is only
	// to make sure there are no replays from pruned inferences.
	err := k.validateTimestamp(ctx, components, msg.InferenceId, 60*60)
	if err != nil {
		return err
	}

	// Create SignatureData with the necessary participants and signatures
	sigData := calculations.SignatureData{
		DevSignature:      msg.InferenceId,
		TransferSignature: msg.TransferSignature,
		ExecutorSignature: msg.ExecutorSignature,
		Dev:               requestor,
		TransferAgent:     transferAgent,
		Executor:          executor,
	}

	// Use the generic VerifyKeys function
	err = calculations.VerifyKeys(ctx, components, sigData, k)
	if err != nil {
		k.LogError("FinishInference: verifyKeys failed", types.Inferences, "error", err)
		return err
	}

	return nil
}

func getFinishSignatureComponents(msg *types.MsgFinishInference) calculations.SignatureComponents {
	return calculations.SignatureComponents{
		Payload:         msg.OriginalPrompt,
		Timestamp:       msg.RequestTimestamp,
		TransferAddress: msg.TransferredBy,
		ExecutorAddress: msg.ExecutedBy,
	}
}

func (k msgServer) handleInferenceCompleted(ctx sdk.Context, existingInference *types.Inference) error {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"inference_finished",
			sdk.NewAttribute("inference_id", existingInference.InferenceId),
		),
	)

	executedBy := existingInference.ExecutedBy
	executor, found := k.GetParticipant(ctx, executedBy)
	if !found {
		k.LogError("handleInferenceCompleted: executor not found", types.Inferences, "executed_by", executedBy)
	} else {
		executor.CurrentEpochStats.InferenceCount++
		executor.LastInferenceTime = existingInference.EndBlockTimestamp
		k.SetParticipant(ctx, executor)
	}

	effectiveEpoch, found := k.GetEffectiveEpoch(ctx)
	if !found {
		k.LogError("Effective Epoch Index not found", types.EpochGroup)
		return types.ErrEffectiveEpochNotFound.Wrapf("handleInferenceCompleted: Effective Epoch Index not found")
	}
	currentEpochGroup, err := k.GetEpochGroupForEpoch(ctx, *effectiveEpoch)
	if err != nil {
		k.LogError("Unable to get current Epoch Group", types.EpochGroup, "err", err)
		return err
	}

	existingInference.EpochPocStartBlockHeight = uint64(effectiveEpoch.PocStartBlockHeight)
	existingInference.EpochId = effectiveEpoch.Index
	currentEpochGroup.GroupData.NumberOfRequests++

	executorPower := uint64(0)
	executorReputation := int32(0)
	for _, weight := range currentEpochGroup.GroupData.ValidationWeights {
		if weight.MemberAddress == existingInference.ExecutedBy {
			executorPower = uint64(weight.Weight)
			executorReputation = weight.Reputation
			break
		}
	}

	modelEpochGroup, err := currentEpochGroup.GetSubGroup(ctx, existingInference.Model)
	if err != nil {
		k.LogError("Unable to get model Epoch Group", types.EpochGroup, "err", err)
		return err
	}

	inferenceDetails := types.InferenceValidationDetails{
		InferenceId:          existingInference.InferenceId,
		ExecutorId:           existingInference.ExecutedBy,
		ExecutorReputation:   executorReputation,
		TrafficBasis:         uint64(math.Max(currentEpochGroup.GroupData.NumberOfRequests, currentEpochGroup.GroupData.PreviousEpochRequests)),
		ExecutorPower:        executorPower,
		EpochId:              effectiveEpoch.Index,
		Model:                existingInference.Model,
		TotalPower:           uint64(modelEpochGroup.GroupData.TotalWeight),
		CreatedAtBlockHeight: ctx.BlockHeight(),
	}
	if inferenceDetails.TotalPower == inferenceDetails.ExecutorPower {
		k.LogWarn("Executor Power equals Total Power", types.Validation,
			"model", existingInference.Model,
			"epoch_id", currentEpochGroup.GroupData.EpochGroupId,
			"epoch_start_block_height", currentEpochGroup.GroupData.PocStartBlockHeight,
			"group_id", modelEpochGroup.GroupData.EpochGroupId,
			"inference_id", existingInference.InferenceId,
			"executor_id", inferenceDetails.ExecutorId,
			"executor_power", inferenceDetails.ExecutorPower,
		)
	}
	k.LogDebug(
		"Adding Inference Validation Details",
		types.Validation,
		"inference_id", inferenceDetails.InferenceId,
		"epoch_id", inferenceDetails.EpochId,
		"executor_id", inferenceDetails.ExecutorId,
		"executor_power", inferenceDetails.ExecutorPower,
		"executor_reputation", inferenceDetails.ExecutorReputation,
		"traffic_basis", inferenceDetails.TrafficBasis,
	)
	k.SetInferenceValidationDetails(ctx, inferenceDetails)
	err = k.SetInference(ctx, *existingInference)
	if err != nil {
		return err
	}
	k.SetEpochGroupData(ctx, *currentEpochGroup.GroupData)
	return nil
}
