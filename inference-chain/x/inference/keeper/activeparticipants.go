package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"encoding/hex"
	"errors"
	"github.com/cosmos/cosmos-sdk/runtime"
	externalutils "github.com/gonka-ai/gonka-utils/go/utils"
	"github.com/productscience/common"
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

func (k Keeper) SetActiveParticipants(ctx context.Context, participants types.ActiveParticipants) error {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte{})

	key := types.ActiveParticipantsFullKey(participants.EpochId)

	b, err := k.cdc.Marshal(&participants)
	if err != nil {
		return err
	}
	store.Set(key, b)
	return nil
}

func (k Keeper) SetActiveParticipantsMerkleProof(ctx context.Context, proof types.ProofOps, blockHeight uint64) error {
	exists, err := k.ActiveParticipantsMerkleProofs.Has(ctx, blockHeight)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	participants, found := k.GetActiveParticipants(ctx, proof.Epoch)
	if !found {
		return errors.New("active participants not found")
	}

	bytes, err := k.cdc.Marshal(&participants)
	if err != nil {
		return err
	}

	out := common.ToCryptoProofOps(&proof)

	// block_proof is created on height N+1 in BeginBlock (on-chain) for each active_participants_set created on height N
	// validators proof is formed on height N+2 (because there is LastCommit for block_height N+1), so block_proof always will be already there when validators_proof comes
	// and since bock_proof formed on-chain, we can trust it
	blockProof, found := k.GetBlockProof(ctx, int64(blockHeight))
	if !found {
		return errors.New("block proof not found")
	}

	hash, err := hex.DecodeString(blockProof.AppHashHex)
	if err != nil {
		return err
	}

	if err := externalutils.VerifyIAVLProofAgainstAppHash(int64(blockHeight), hash, out.Ops, bytes); err != nil {
		return err
	}

	return k.ActiveParticipantsMerkleProofs.Set(ctx, blockHeight, proof)
}

func (k Keeper) GetActiveParticipantsProof(ctx context.Context, blockHeight int64) (types.ProofOps, bool) {
	v, err := k.ActiveParticipantsMerkleProofs.Get(ctx, uint64(blockHeight))
	if err != nil {
		return types.ProofOps{}, false
	}
	return v, true
}
