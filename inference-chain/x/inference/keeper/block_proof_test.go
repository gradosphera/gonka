package keeper_test

import (
	keepertest "github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBlockProof(t *testing.T) {
	k, ctx, _ := keepertest.InferenceKeeperReturningMocks(t)
	k.SetActiveParticipants(ctx, types.ActiveParticipants{
		Participants: []*types.ActiveParticipant{
			{ValidatorKey: "BUWZfCeWI3O+UXcmCbnjacmi0RY0PzX/8aJKdy3rP48="},
			{ValidatorKey: "pM9MGrvN6zoLAuA6SKndq2GT/AY8b9tr8PodsnmV4Bk="},
		},
	})

	t.Run("get not existing block/pending proof", func(t *testing.T) {
		const height = 123
		_, found := k.GetBlockProof(ctx, height)
		assert.False(t, found)

		_, found = k.GetPendingProof(ctx, height)
		assert.False(t, found)
	})

	t.Run("try to set proof with empty commits", func(t *testing.T) {
		err := k.SetBlockProof(ctx, types.BlockProof{
			CreatedAtBlockHeight: 10,
			AppHashHex:           "apphash-10",
			TotalVotingPower:     100,
			EpochId:              1,
		})
		assert.ErrorIs(t, err, keeper.ErrEmptyCommits)
	})

	t.Run("try to set proof with wrong validator key", func(t *testing.T) {
		err := k.SetBlockProof(ctx, types.BlockProof{
			CreatedAtBlockHeight: 10,
			AppHashHex:           "apphash-10",
			TotalVotingPower:     100,
			EpochId:              1,
			Commits: []*types.CommitInfo{
				{
					ValidatorPubKey: "some_key",
				},
			},
		})
		assert.ErrorContains(t, err, "commit validator address not found in participants")
	})

	t.Run("try to set proof with wrong validator key", func(t *testing.T) {
		err := k.SetBlockProof(ctx, types.BlockProof{
			CreatedAtBlockHeight: 10,
			AppHashHex:           "apphash-10",
			TotalVotingPower:     100,
			EpochId:              1,
			Commits: []*types.CommitInfo{
				{
					ValidatorAddress: "901ADC33D3A63CBF9EF17B8CB5F04F99087D47E0",
					ValidatorPubKey:  "BUWZfCeWI3O+UXcmCbnjacmi0RY0PzX/8aJKdy3rP49=",
				},
			},
		})
		assert.ErrorContains(t, err, "commit validator key and participant validator key are not matching")
	})

	t.Run("set block proof", func(t *testing.T) {
		const height = 10
		proof := types.BlockProof{
			CreatedAtBlockHeight: height,
			AppHashHex:           "apphash-10",
			TotalVotingPower:     100,
			EpochId:              1,
			Commits: []*types.CommitInfo{
				{
					ValidatorAddress: "901ADC33D3A63CBF9EF17B8CB5F04F99087D47E0",
					ValidatorPubKey:  "BUWZfCeWI3O+UXcmCbnjacmi0RY0PzX/8aJKdy3rP48=",
				},
			},
		}

		err := k.SetBlockProof(ctx, proof)
		assert.NoError(t, err)

		got, found := k.GetBlockProof(ctx, height)
		assert.True(t, found)
		assert.Equal(t, proof.CreatedAtBlockHeight, got.CreatedAtBlockHeight)
		assert.Equal(t, proof.AppHashHex, got.AppHashHex)
		assert.Equal(t, proof.TotalVotingPower, got.TotalVotingPower)

		err = k.SetBlockProof(ctx, proof)
		assert.NoError(t, err)
	})

	t.Run("get pending proof", func(t *testing.T) {
		h := int64(20)
		_, found := k.GetPendingProof(ctx, h)
		assert.False(t, found)

		epoch := uint64(345)
		err := k.SetPendingProof(ctx, h, epoch)
		assert.NoError(t, err)

		pendingProofEpochId, found := k.GetPendingProof(ctx, h)
		assert.True(t, found)
		assert.Equal(t, epoch, pendingProofEpochId)

		err = k.SetPendingProof(ctx, h, 123214)
		assert.NoError(t, err)

		pendingProofEpochId, found = k.GetPendingProof(ctx, h)
		assert.True(t, found)
		assert.Equal(t, epoch, pendingProofEpochId)
	})
}
