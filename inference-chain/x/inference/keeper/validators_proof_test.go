package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/types"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSetValidatorsProof(t *testing.T) {
	keeper, ctx := keepertest.InferenceKeeper(t)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx = sdkCtx.WithChainID("gonka-mainnet")
	ctx2 := sdk.WrapSDKContext(sdkCtx)

	t1, err := time.Parse(time.RFC3339Nano, "2025-08-23T12:04:16.634850393Z")
	assert.NoError(t, err)

	t2, err := time.Parse(time.RFC3339Nano, "2025-08-23T12:04:16.601168243Z")
	assert.NoError(t, err)

	blockHash := "0B902548DF9480890973D4F085AED92D7A5D64E132BE4FCFD76EB472973170C2"
	partHash := "78AAB0B9F08B50C5E7AE70C0376137A195F4AC1B08208E8B7F152779D25AB491"

	const validProofEpoch2BlockHeight = int64(17665)
	validProofEpoch2 := types.ValidatorsProof{
		BlockHeight: validProofEpoch2BlockHeight,
		Round:       0,
		BlockId: &types.BlockID{
			Hash:               blockHash,
			PartSetHeaderTotal: 1,
			PartSetHeaderHash:  partHash,
		},
		Signatures: []*types.SignatureInfo{
			{
				SignatureBase64:     "LDIifb8QUyz3koJ6674F5iZ0WfJWmLTxCth/BJKp4auRH8hGeOz6MfPy+DERfQz6+AQzj/caPqImiAOIQArqBw==",
				ValidatorAddressHex: "13CCD46D7FF3BB4945A2FBC450A948B5A1C89EB9",
				Timestamp:           t1,
			},
			{
				SignatureBase64:     "SHfLV0c8r4ikqjwZ+xcopW0DOei0ARufdSymlVtYvlSd0SXRv9+uVfJ14dUT74HPh4hryeTisPy3cw7dXASbBw==",
				ValidatorAddressHex: "DF04B29653F664BDAC7DE851D52BDD5C8E205822",
				Timestamp:           t2,
			},
		},
	}

	// 1. Try insert validators proof BEFORE block proof is set -> fail
	err = keeper.SetValidatorsProof(ctx2, validProofEpoch2)
	assert.ErrorContains(t, err, "block proof not found")

	// 2. Try to set invalid validators proof, but store valid block proof before -> fail
	err = keeper.SetBlockProof(ctx2, types.BlockProof{
		CreatedAtBlockHeight: validProofEpoch2BlockHeight,
		AppHashHex:           "29213E6A386DE8F6BA3882A87490347B91D8C9B3D63FBBEB2400A2967B5F6939",
		Commits: []*types.CommitInfo{
			{
				ValidatorAddress: "13CCD46D7FF3BB4945A2FBC450A948B5A1C89EB9",
				ValidatorPubKey:  "LLqBxOz+vD3p7sQsdEhBfrFH2QFMjy3fMasB9yBGSqs=",
			},
			{
				ValidatorAddress: "DF04B29653F664BDAC7DE851D52BDD5C8E205822",
				ValidatorPubKey:  "5QYFI0kdyBPrcld3FfOwoZdynfwN5li0qUbg3zwFK4I=",
			},
		},
	})
	assert.NoError(t, err)
	invalidProof := types.ValidatorsProof{
		BlockHeight: validProofEpoch2BlockHeight,
		Round:       0,
		BlockId: &types.BlockID{
			Hash:               blockHash,
			PartSetHeaderTotal: 1,
			PartSetHeaderHash:  partHash,
		},
		Signatures: []*types.SignatureInfo{
			{
				SignatureBase64:     "LDIifb8QUyz3koJ6674F5iZ0WfJWmLTxCth/BJKp4auRH8hGeOz6MfPy+DERfQz6+AQzj/caPqImiAOIQArqBw==",
				ValidatorAddressHex: "13CCD46D7FF3BB4945A2FBC450A948B5A1C89EB9",
				Timestamp:           t1,
			},
			{
				SignatureBase64:     "SHfLV0c8r4ikqjwZ+xcopW0DOei0ARufdSymlVtYvlSd0SXRv9+uVfJ14dUT74HPh4hryeTisPy3cw7dXASbBw==",
				ValidatorAddressHex: "DF04B29653F664BDAC7DE851D52BDD5C8E205822",
				Timestamp:           time.Now(), // wrong ts makes signature invalid
			},
		},
	}

	err = keeper.SetValidatorsProof(ctx2, invalidProof)
	assert.ErrorContains(t, err, "failed to verify signature")

	_, found := keeper.GetValidatorsProof(ctx2, validProofEpoch2BlockHeight)
	assert.False(t, found)

	// 3. Set correct proof
	err = keeper.SetValidatorsProof(ctx2, validProofEpoch2)
	assert.NoError(t, err)

	actualProof, found := keeper.GetValidatorsProof(ctx2, validProofEpoch2BlockHeight)
	assert.True(t, found)
	assert.Equal(t, validProofEpoch2, actualProof)

	// 4. Try to re-write proof with same height -> data still same
	err = keeper.SetValidatorsProof(ctx2, types.ValidatorsProof{
		BlockHeight: validProofEpoch2BlockHeight,
		Round:       2,
	})
	assert.NoError(t, err)

	actualProof, found = keeper.GetValidatorsProof(ctx2, validProofEpoch2BlockHeight)
	assert.True(t, found)
	assert.Equal(t, validProofEpoch2, actualProof)
}
