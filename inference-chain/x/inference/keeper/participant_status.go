package keeper

import (
	"context"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/calculations"
	"github.com/productscience/inference/x/inference/types"
	"github.com/shopspring/decimal"
)

// UpdateParticipantStatus is the single entry point for changing a participant's status.
// It detects transitions and applies side-effects exactly once. Currently, when transitioning
// to INVALID it will: slash collateral, record an exclusion entry for the current epoch,
// and invoke removal from EpochGroup memberships for the current epoch.
func (k Keeper) UpdateParticipantStatus(ctx context.Context, participant *types.Participant) error {
	if participant == nil {
		return nil
	}

	params := k.GetParams(ctx)
	originalStatus := participant.Status
	newStatus, reason := calculations.ComputeStatus(params.ValidationParams, *participant)

	if originalStatus == newStatus {
		return nil
	}

	// This should be the ONLY place status is set
	participant.Status = newStatus

	// Handle transition to INVALID once.
	if originalStatus != types.ParticipantStatus_INVALID && newStatus == types.ParticipantStatus_INVALID {
		return k.invalidateParticipant(ctx, participant, reason, params)
	}

	return nil
}

// invalidateParticipant performs all side-effects associated with a participant becoming INVALID.
// This includes:
// - Slashing collateral according to params.CollateralParams.SlashFractionInvalid
// - Recording an ExcludedParticipants entry for the current effective epoch
// - Removing the participant from the EpochGroup parent and all model sub-groups for the current epoch
// Idempotency: Recording to ExcludedParticipants uses Set with (epoch_index, address) composite key.
func (k Keeper) invalidateParticipant(ctx context.Context, participant *types.Participant, reason calculations.ParticipantStatusReason, params types.Params) error {
	// 1) Slash collateral
	// TODO: Slash, unlike the other two, is NOT idempotent! We need checks to make sure it is
	k.SlashForInvalidStatus(ctx, participant, params)

	// 2) Record exclusion entry for current effective epoch (if available)
	k.recordExclusion(ctx, participant, reason)

	// 3) TODO: Multiply EpochsCompleted by the ReputationPreserve
	participant.EpochsCompleted = multiply(participant.EpochsCompleted, params.ValidationParams.InvalidReputationPreserve)

	// 4) Remove from current-epoch EpochGroup memberships
	return k.removeFromEpochGroups(ctx, participant, reason)
}

// This is way messier than you'd expect...
func multiply(completed uint32, preserve *types.Decimal) uint32 {
	if preserve == nil {
		return completed
	}
	pd := preserve.ToDecimal()
	if pd.LessThan(decimal.Zero) || pd.GreaterThan(decimal.NewFromInt(1)) {
		return completed
	}
	toDecimal := preserve.ToDecimal()
	result := decimal.NewFromInt32(int32(completed)).Mul(toDecimal)
	return uint32(result.Round(0).IntPart())
}

func (k Keeper) removeFromEpochGroups(ctx context.Context, participant *types.Participant, reason calculations.ParticipantStatusReason) error {
	parentGroup, err := k.GetCurrentEpochGroup(ctx)
	if err != nil {
		k.LogError("Failed to get current epoch group", types.Validation, "error", err)
		return err
	}
	return parentGroup.RemoveMember(ctx, participant)
}

func (k Keeper) recordExclusion(ctx context.Context, participant *types.Participant, reason calculations.ParticipantStatusReason) {
	if epochIndex, ok := k.GetEffectiveEpochIndex(ctx); ok {
		addr, err := sdk.AccAddressFromBech32(participant.Address)
		if err == nil {
			_ = k.ExcludedParticipantsMap.Set(ctx, collections.Join(epochIndex, addr), types.ExcludedParticipant{
				Address:         participant.Address,
				EpochIndex:      epochIndex,
				Reason:          string(reason),
				EffectiveHeight: uint64(sdk.UnwrapSDKContext(ctx).BlockHeight()),
			})
		} else {
			k.LogError("Failed to parse participant address for exclusion entry", types.Validation, "address", participant.Address, "error", err)
		}
	}
}
