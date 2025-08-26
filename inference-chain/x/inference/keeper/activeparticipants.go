package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/productscience/inference/x/inference/types"
)

func (k Keeper) SetActiveParticipantsV1(ctx context.Context, participants types.ActiveParticipants) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte{})

	key := types.ActiveParticipantsFullKeyV1(participants.EpochGroupId)

	b := k.cdc.MustMarshal(&participants)
	store.Set(key, b)
}

func (k Keeper) GetActiveParticipants(ctx context.Context, epochId uint64) (val types.ActiveParticipants, found bool) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte{})

	key := types.ActiveParticipantsFullKey(epochId)

	b := store.Get(key)
	if b == nil {
		return types.ActiveParticipants{}, false
	}

	err := k.cdc.Unmarshal(b, &val)
	if err != nil {
		k.LogError("failed to unmarshal active participants", types.Participants, "error", err)
		return types.ActiveParticipants{}, false
	}
	return val, true
}

func (k Keeper) SetActiveParticipants(ctx context.Context, participants types.ActiveParticipants) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte{})

	key := types.ActiveParticipantsFullKey(participants.EpochId)

	b := k.cdc.MustMarshal(&participants)
	store.Set(key, b)
}

func (k Keeper) SetActiveParticipantsProof(ctx context.Context, proof types.ProofOps, blockHeight uint64) error {
	exists, err := k.ActiveParticipantsProofs.Has(ctx, blockHeight)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return k.ActiveParticipantsProofs.Set(ctx, blockHeight, proof)
}

func (k Keeper) GetActiveParticipantsProof(ctx context.Context, blockHeight int64) (types.ProofOps, bool) {
	v, err := k.ActiveParticipantsProofs.Get(ctx, uint64(blockHeight))
	if err != nil {
		return types.ProofOps{}, false
	}
	return v, true
}
