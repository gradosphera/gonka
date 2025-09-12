package keeper

import (
	"context"
	"errors"
	"github.com/productscience/common"
	"github.com/productscience/inference/x/inference/types"
	"strings"
)

var ErrEmptyCommits = errors.New("empty commits")

func (k Keeper) SetBlockProof(ctx context.Context, proof types.BlockProof) error {
	h := uint64(proof.CreatedAtBlockHeight)
	if h == 0 {
		return ErrEmptyBlockHeight
	}

	if len(proof.Commits) == 0 {
		return ErrEmptyCommits
	}

	// verify validators, which signed this block: they all must be in active participants set
	var (
		prevParticipants types.ActiveParticipants
		found            bool
	)

	if proof.EpochId == 0 {
		prevParticipants, found = k.GetActiveParticipants(ctx, proof.EpochId)
	} else {
		epoch := proof.EpochId - 1
		prevParticipants, found = k.GetActiveParticipants(ctx, epoch)
	}

	if !found {
		k.logger.Error("participants not found for previous epoch")
		return ErrParticipantsNotFound
	}

	participantsData := make(map[string]string)
	for _, participant := range prevParticipants.Participants {
		addrHex, err := common.ConsensusKeyToConsensusAddress(participant.ValidatorKey)
		if err != nil {
			return err
		}
		participantsData[strings.ToUpper(addrHex)] = participant.ValidatorKey
	}

	for _, commit := range proof.Commits {
		key, ok := participantsData[strings.ToUpper(commit.ValidatorAddress)]
		if !ok {
			return errors.New("commit validator address not found in participants")
		}
		if strings.ToUpper(key) != strings.ToUpper(commit.ValidatorPubKey) {
			return errors.New("commit validator key and participant validator key are not matching")
		}
	}

	return k.BlockProofs.Set(ctx, h, proof)
}

func (k Keeper) GetBlockProof(ctx context.Context, height int64) (types.BlockProof, bool) {
	v, err := k.BlockProofs.Get(ctx, uint64(height))
	if err != nil {
		return types.BlockProof{}, false
	}
	return v, true
}

func (k Keeper) SetPendingProof(ctx context.Context, height int64, participantsEpoch uint64) error {
	h := uint64(height)

	exists, err := k.PendingProofs.Has(ctx, h)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return k.PendingProofs.Set(ctx, h, participantsEpoch)
}

func (k Keeper) GetPendingProof(ctx context.Context, height int64) (uint64, bool) {
	v, err := k.PendingProofs.Get(ctx, uint64(height))
	if err != nil {
		return 0, false
	}
	return v, true
}
