package keeper

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	externalutils "github.com/gonka-ai/gonka-utils/go/utils"
	"github.com/productscience/common"
	"github.com/productscience/inference/x/inference/types"
)

func (k Keeper) SetValidatorsProof(ctx context.Context, proof types.ValidatorsProof) error {
	h := uint64(proof.BlockHeight)

	exists, err := k.ValidatorsProofs.Has(ctx, h)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// block_proof is created on height N+1 in BeginBlock (on-chain) for each active_participants_set created on height N
	// validators proof is formed on height N+2 (because there is LastCommit for block_height N+1), so block_proof always will be already there when validators_proof comes
	// and since bock_proof formed on-chain, we can trust it
	blockProof, found := k.GetBlockProof(ctx, int64(h))
	if !found {
		return fmt.Errorf("block proof not found for height %v", h)
	}

	validatorsData := make(map[string]string)
	for _, commit := range blockProof.Commits {
		validatorsData[commit.ValidatorAddress] = commit.ValidatorPubKey
	}

	out := common.ToContractsValidatorsProof(&proof)
	if err := externalutils.VerifySignatures(*out, sdkCtx.ChainID(), validatorsData); err != nil {
		return err
	}
	return k.ValidatorsProofs.Set(ctx, h, proof)
}

func (k Keeper) GetValidatorsProof(ctx context.Context, height int64) (types.ValidatorsProof, bool) {
	v, err := k.ValidatorsProofs.Get(ctx, uint64(height))
	if err != nil {
		return types.ValidatorsProof{}, false
	}
	return v, true
}
