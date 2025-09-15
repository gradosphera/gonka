package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/types"
)

func (k msgServer) InvalidateInference(goCtx context.Context, msg *types.MsgInvalidateInference) (*types.MsgInvalidateInferenceResponse, error) {
	inference, found := k.GetInference(goCtx, msg.InferenceId)
	if found != true {
		k.LogError("Inference not found", types.Validation, "inferenceId", msg.InferenceId)
		return nil, errorsmod.Wrapf(types.ErrInferenceNotFound, "inference with id %s not found", msg.InferenceId)
	}

	if msg.Creator != inference.ProposalDetails.PolicyAddress {
		k.LogError("Invalid authority", types.Validation, "expected", inference.ProposalDetails.PolicyAddress, "got", msg.Creator)
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", inference.ProposalDetails.PolicyAddress, msg.Creator)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	executor, found := k.GetParticipant(ctx, inference.ExecutedBy)
	if found != true {
		k.LogError("Participant not found", types.Validation, "address", inference.ExecutedBy)
		return nil, errorsmod.Wrapf(types.ErrParticipantNotFound, "participant with address %s not found", inference.ExecutedBy)
	}

	// Idempotent, so no error
	if inference.Status == types.InferenceStatus_INVALIDATED {
		k.LogDebug("Inference already invalidated", types.Validation, "inferenceId", msg.InferenceId)
		return nil, nil
	}
	inference.Status = types.InferenceStatus_INVALIDATED
	executor.CurrentEpochStats.InvalidatedInferences++
	executor.ConsecutiveInvalidInferences++
	epochGroup, err := k.GetCurrentEpochGroup(ctx)
	if err != nil {
		k.LogError("Failed to get current epoch group", types.Validation, "error", err)
		return nil, err
	}

	shouldRefund, reason := k.inferenceIsBeforeClaimsSet(ctx, inference, epochGroup)
	k.LogInfo("Inference refund decision", types.Validation, "inferenceId", inference.InferenceId, "executor", executor.Address, "shouldRefund", shouldRefund, "reason", reason)
	if shouldRefund {
		err := k.refundInvalidatedInference(&executor, &inference, ctx)
		if err != nil {
			return nil, err
		}
	}

	k.LogInfo("Inference invalidated", types.Inferences, "inferenceId", inference.InferenceId, "executor", executor.Address, "actualCost", inference.ActualCost)

	// Store the original status to check for a state transition to INVALID.
	originalStatus := executor.Status
	executor.Status = calculateStatus(k.Keeper.GetParams(goCtx).ValidationParams, executor)

	// Check for a status transition and slash if necessary.
	k.CheckAndSlashForInvalidStatus(goCtx, originalStatus, &executor)

	k.SetInference(ctx, inference)
	k.SetParticipant(ctx, executor)
	return &types.MsgInvalidateInferenceResponse{}, nil
}

func (k msgServer) refundInvalidatedInference(executor *types.Participant, inference *types.Inference, ctx sdk.Context) error {
	executor.CoinBalance -= inference.ActualCost
	k.SafeLogSubAccountTransaction(ctx, types.ModuleName, executor.Address, types.OwedSubAccount, inference.ActualCost, "inference_invalidated:"+inference.InferenceId)
	k.LogInfo("Invalid Inference subtracted from Executor CoinBalance ", types.Balances, "inferenceId", inference.InferenceId, "executor", executor.Address, "actualCost", inference.ActualCost, "coinBalance", executor.CoinBalance)
	// We need to refund the cost, so we have to lookup the person who paid
	payer, found := k.GetParticipant(ctx, inference.RequestedBy)
	if !found {
		k.LogError("Payer not found", types.Validation, "address", inference.RequestedBy)
		return types.ErrParticipantNotFound
	}
	err := k.IssueRefund(ctx, inference.ActualCost, payer.Address, "invalidated_inference:"+inference.InferenceId)
	if err != nil {
		k.LogError("Refund failed", types.Validation, "error", err)
	}
	return nil
}
