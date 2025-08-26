package keeper

import (
	"context"

	"cosmossdk.io/collections"
	"github.com/productscience/inference/x/inference/types"
)

func (k Keeper) SetBlockProof(ctx context.Context, proof types.BlockProof) error {
	h := uint64(proof.CreatedAtBlockHeight)

	exists, err := k.BlockProofs.Has(ctx, h)
	if err != nil {
		return err
	}
	if exists {
		return collections.ErrConflict
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
