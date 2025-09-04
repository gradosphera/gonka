package keeper_test

import (
	"encoding/base64"
	"encoding/hex"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/testutil/keeper"
	"github.com/productscience/inference/x/inference/types"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSubmitMissingParticipantsProofData(t *testing.T) {
	const (
		epoch1ActiveParticipantsHeight = 59
		epoch0ActiveParticipantsHeight = 1
		currentEpoch                   = 1
	)

	validProofOpsEpoch1 := types.ProofOps{
		Epoch:       currentEpoch,
		BlockHeight: epoch1ActiveParticipantsHeight,
		Ops: []types.ProofOp{
			{
				Type: "ics23:iavl",
				Key:  mustDecodeBase64(t, "QWN0aXZlUGFydGljaXBhbnRzL3ZhbHVlLwAAAAAAAAABLw=="),
				Data: mustDecodeBase64(t, "CtEHCiJBY3RpdmVQYXJ0aWNpcGFudHMvdmFsdWUvAAAAAAAAAAEvEsQFCtsCCixnb25rYTE4anZyenNsemd6eXowZzRzY2VjM2w3ZjhnOGhxMDVjMDk0eWdtORIsRERISGNCQmtjRzQxS1I3SkxQVGovU2RaSUh1K0JxcWNSTjEvVklHb2hFRT0YCiIXaHR0cDovL2dlbmVzaXMtYXBpOjkwMDAqGFF3ZW4vUXdlbjIuNS03Qi1JbnN0cnVjdDKzAQosZ29ua2ExOGp2cnpzbHpnenl6MGc0c2NlYzNsN2Y4ZzhocTA1YzA5NHlnbTkQARqAATY5OTc5NzI0MGM4MmUzNDUwNDAyMzAxZDdkNmE5ZWFiZWE4ZjU4NTgxMjY1NmI3NDBmZDQ2MjVjNGE5OWY5NmMxNTJkNGI5ZjVkMTM2MDUyOWM3OGJiM2U4MmI5YmYyYTYyMDJkODBiOTkwNDY3MTNhMTkyYzRkNjU1NWVjYzQ1OhIKEAoId2lyZW1vY2sYCiICAQAK2QIKLGdvbmthMXRrY21mc2oyZG0wMnNzOHhuMmNsM3BheTJybDZzNHo2NDIzeDZhEixRQWhNM003bFEzb00rdGFVRy90dU1TaE1jaGM2YTNxR2tRek82eVR1dW1ZPRgKIhVodHRwOi8vam9pbjEtYXBpOjkwMTAqGFF3ZW4vUXdlbjIuNS03Qi1JbnN0cnVjdDKzAQosZ29ua2ExdGtjbWZzajJkbTAyc3M4eG4yY2wzcGF5MnJsNnM0ejY0MjN4NmEQARqAAWVkOWEyMDZhYzI2YjQwM2E2NmRmZjY4ODM4YzEyOGFmYWRkZWU4OGRlNmNhZjYwNGYxOWNlMjU5ZTkzZTNjNjk3NTA0ZGQ1NzQzMTJhMDlhNTVmMDcxNzk1OTc1N2I3M2FhNjAzZTBlOWViM2QzNDkzNjEwYzY4ZjUyNjRhZTQ3OhIKEAoId2lyZW1vY2sYCiICAQAQARgyID0oOzABGgsIARgBIAEqAwACdiIpCAESJQIEdiDTM5juv1Mz8ZCj78r1kbhbFiPHOixSLJA5Dymhi2fp5iAiKQgBEiUEBnYg0vutpR6TMMeUo1fPMNIAjm6ZH0t6GjKPlRw8x9GZsmkgIikIARIlBgp2INVefcIpRMYCQr57GZ55leHfAe2rD4VyE4jUjx0K+CnjICIrCAESBAgWdiAaISDTo5n/k4gkHnO+80UL/gJ4EgDEXdibeUVU/0KZUQiB0iIpCAESJQw8diABj/XToD2SnV7mntGWeR5NiCwsAS4B6NHpiRPMwEezQCA="),
			},
			{
				Type: "ics23:simple",
				Key:  mustDecodeBase64(t, "aW5mZXJlbmNl"),
				Data: mustDecodeBase64(t, "CoECCglpbmZlcmVuY2USIFc7e18r5zB3vh88LQtcQbi2z3F/JAX+zam3pCL1yplrGgkIARgBIAEqAQAiJwgBEgEBGiDYIBUQfvVdZo0xMFSpQCS48dWzGYMmqq6ExQUPvM2+6yInCAESAQEaIJXhyLpHqjGG8RNH1IDYExUpZFPyxZvc7HLLmYEKxBeAIiUIARIhAZQlD60D4tpnZB/Cb2zR/tBsixvO9zdekA3JOoI9uG0XIicIARIBARog+/sWb5lSJgLxQngZOcyx7M25LV2qAN3y1K+x0njvDBkiJQgBEiEBqOwfS96LcWN4+AeJdrNuO1rvr+sKiTG38eHfMVcDjIg="),
			},
		},
	}

	validActiveParticiapntsEpoch0 := types.ActiveParticipants{
		Participants: []*types.ActiveParticipant{
			{
				Index:        "gonka18jvrzslzgzyz0g4scec3l7f8g8hq05c094ygm9",
				ValidatorKey: "DDHHcBBkcG41KR7JLPTj/SdZIHu+BqqcRN1/VIGohEE=",
				Weight:       10,
				InferenceUrl: "http://genesis-api:9000",
				Models:       []string{"Qwen/Qwen2.5-7B-Instruct"},
				Seed: &types.RandomSeed{
					Participant: "gonka18jvrzslzgzyz0g4scec3l7f8g8hq05c094ygm9",
					EpochIndex:  1,
					Signature:   "699797240c82e3450402301d7d6a9eabea8f585812656b740fd4625c4a99f96c152d4b9f5d1360529c78bb3e82b9bf2a6202d80b99046713a192c4d6555ecc45",
				},
				MlNodes: []*types.ModelMLNodes{
					{
						MlNodes: []*types.MLNodeInfo{
							{
								NodeId:             "wiremock",
								Throughput:         0,
								PocWeight:          10,
								TimeslotAllocation: []bool{true, false},
							},
						},
					},
				},
			},
		},
		CreatedAtBlockHeight: epoch0ActiveParticipantsHeight,
	}
	validHeaderEpoch1 := types.BlockHeaderFull{
		Version:   11,
		ChainId:   "gonka-mainnet",
		Height:    epoch1ActiveParticipantsHeight + 1,
		Timestamp: mustParseTime(t, "2025-09-04T13:10:33.221203044Z"),
		LastBlockId: &types.BlockID{
			Hash:               "B4D21ECD64711EDAD1D370199089694664445BAB52E9AC44FA54FD44E2FD00B6",
			PartSetHeaderTotal: 1,
			PartSetHeaderHash:  "BD1819589084ED21FF797C61313B77FB794F60BB1A1359FDA9F52B3E34BD4399",
		},
		LastCommitHash:     mustDecodeHex(t, "B09243D5EB4DA7C8A1AD1FEAD380FDFFB3B10A492A02E573E8A579645A808CFA"),
		DataHash:           mustDecodeHex(t, "4C8FCBFC930092BD829210CBE0990018DF62F136CDAC0AA3B29B652BD298160E"),
		ValidatorsHash:     mustDecodeHex(t, "ED10CFC1F26B54E9FF0E568263AFB7E219010EB6AB9A51BAE44B74CAA95A49E8"),
		NextValidatorsHash: mustDecodeHex(t, "ED10CFC1F26B54E9FF0E568263AFB7E219010EB6AB9A51BAE44B74CAA95A49E8"),
		ConsensusHash:      mustDecodeHex(t, "048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F"),
		AppHash:            mustDecodeHex(t, "40768F2C7C3A5816A5FEA6DCCCD887C8778C05AC2693473947839DAE8CF3FDF9"),
		LastResultsHash:    mustDecodeHex(t, "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"),
		EvidenceHash:       mustDecodeHex(t, "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"),
		ProposerAddress:    mustDecodeHex(t, "7991A95AD2C4906F847C9EDA58E965C88265C4BD"),
	}

	validCurrentValidatorsProof := types.ValidatorsProof{
		BlockHeight: epoch1ActiveParticipantsHeight,
		Round:       0,
		BlockId:     validHeaderEpoch1.LastBlockId,
		Signatures: []*types.SignatureInfo{
			{
				SignatureBase64:     "TNedbJabG7GgoPaoP57Dib2S0NDbkiL0/gZQaptTfWHpNL7L3CGGL+IsWSn1dAfKk+/wnLoKrpOPui4lE+YaBw==",
				ValidatorAddressHex: "7991A95AD2C4906F847C9EDA58E965C88265C4BD",
				Timestamp:           mustParseTime(t, "2025-09-04T13:10:33.221203044Z"),
			},
		},
	}

	validNextBlockValidatorsProof := types.ValidatorsProof{
		BlockHeight: epoch1ActiveParticipantsHeight + 1,
		BlockId: &types.BlockID{
			PartSetHeaderTotal: 1,
			PartSetHeaderHash:  "AA87D916F3CFAF8722021F6FAA15C2252A21C9F3397E85C23F2A09CA50FE0F0E",
		},
		Signatures: []*types.SignatureInfo{
			{
				SignatureBase64:     "M4+oQVvY5HetDMnLnkjR8JVkHRP6gz2CTMjiNr/9DN2oAQw0MzsllM5QmZ59lv2r1J1YdPDhRsyIx8vWrppSCg==",
				ValidatorAddressHex: "7991A95AD2C4906F847C9EDA58E965C88265C4BD",
				Timestamp:           mustParseTime(t, "2025-09-04T13:10:38.256296836Z"),
			},
		},
	}

	validCurrentActiveParticiapnts := types.ActiveParticipants{
		Participants: []*types.ActiveParticipant{
			{
				Index:        "gonka18jvrzslzgzyz0g4scec3l7f8g8hq05c094ygm9",
				ValidatorKey: "DDHHcBBkcG41KR7JLPTj/SdZIHu+BqqcRN1/VIGohEE=",
				Weight:       10,
				InferenceUrl: "http://genesis-api:9000",
				Models:       []string{"Qwen/Qwen2.5-7B-Instruct"},
				Seed: &types.RandomSeed{
					Participant: "gonka18jvrzslzgzyz0g4scec3l7f8g8hq05c094ygm9",
					EpochIndex:  1,
					Signature:   "699797240c82e3450402301d7d6a9eabea8f585812656b740fd4625c4a99f96c152d4b9f5d1360529c78bb3e82b9bf2a6202d80b99046713a192c4d6555ecc45",
				},
				MlNodes: []*types.ModelMLNodes{
					{
						MlNodes: []*types.MLNodeInfo{
							{
								NodeId:             "wiremock",
								Throughput:         0,
								PocWeight:          10,
								TimeslotAllocation: []bool{true, false},
							},
						},
					},
				},
			},
			{
				Index:        "gonka1tkcmfsj2dm02ss8xn2cl3pay2rl6s4z6423x6a",
				ValidatorKey: "QAhM3M7lQ3oM+taUG/tuMShMchc6a3qGkQzO6yTuumY=",
				Weight:       10,
				InferenceUrl: "http://join1-api:9010",
				Models:       []string{"Qwen/Qwen2.5-7B-Instruct"},
				Seed: &types.RandomSeed{
					Participant: "gonka1tkcmfsj2dm02ss8xn2cl3pay2rl6s4z6423x6a",
					EpochIndex:  1,
					Signature:   "ed9a206ac26b403a66dff68838c128afaddee88de6caf604f19ce259e93e3c697504dd574312a09a55f0717959757b73aa603e0e9eb3d3493610c68f5264ae47",
				},
				MlNodes: []*types.ModelMLNodes{
					{
						MlNodes: []*types.MLNodeInfo{
							{
								NodeId:             "wiremock",
								Throughput:         0,
								PocWeight:          10,
								TimeslotAllocation: []bool{true, false},
							},
						},
					},
				},
			},
		},
		EpochGroupId:         1,
		PocStartBlockHeight:  50,
		EffectiveBlockHeight: 61,
		CreatedAtBlockHeight: 59,
		EpochId:              1,
	}

	validHeaderEpoch0 := types.BlockHeaderFull{
		Version:   11,
		ChainId:   "gonka-mainnet",
		Height:    epoch0ActiveParticipantsHeight + 1,
		Timestamp: mustParseTime(t, "2025-09-04T13:05:41.29844009Z"),
		LastBlockId: &types.BlockID{
			Hash:               "C6488452BD6B9FCD641AB50652E375797A0445397527FE1749E2F30E3866C3EF",
			PartSetHeaderTotal: 1,
			PartSetHeaderHash:  "2334374149EADF07174BFF63DCD1C87988F650A0F2A3DDCD02EBE5B8F2669E65",
		},
		LastCommitHash:     mustDecodeHex(t, "31546EC1B152017A00DF1B9A27D2780BDEBA374153DE296F47F8E6B2B37C59D8"),
		DataHash:           mustDecodeHex(t, "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"),
		ValidatorsHash:     mustDecodeHex(t, "ED10CFC1F26B54E9FF0E568263AFB7E219010EB6AB9A51BAE44B74CAA95A49E8"),
		NextValidatorsHash: mustDecodeHex(t, "ED10CFC1F26B54E9FF0E568263AFB7E219010EB6AB9A51BAE44B74CAA95A49E8"),
		ConsensusHash:      mustDecodeHex(t, "048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F"),
		AppHash:            mustDecodeHex(t, "2059A07D828A6D9F9D54EC09E92A80919FC229D9CBE306E3BC7EF63D85AC9A2B"),
		LastResultsHash:    mustDecodeHex(t, "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"),
		EvidenceHash:       mustDecodeHex(t, "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"),
		ProposerAddress:    mustDecodeHex(t, "7991A95AD2C4906F847C9EDA58E965C88265C4BD"),
	}

	t.Run("success: store proofs for epoch 1", func(t *testing.T) {
		k, ctx := keeper.InferenceKeeper(t)
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		ctx = sdkCtx.WithChainID("gonka-mainnet")

		k.SetActiveParticipants(ctx, validActiveParticiapntsEpoch0)
		k.SetActiveParticipants(ctx, validCurrentActiveParticiapnts)

		msgSrv := setupMsgServerWithKeeper(k)
		_, err := msgSrv.SubmitMissingParticipantsProofData(ctx, &types.MsgSubmitActiveParticipantsProofData{
			BlockHeight:                 epoch1ActiveParticipantsHeight,
			EpochId:                     currentEpoch,
			CurrentBlockValidatorsProof: &validCurrentValidatorsProof,
			NextBlockValidatorsProof:    &validNextBlockValidatorsProof,
			BlockProof:                  &validHeaderEpoch1,
			ProofOpts:                   &validProofOpsEpoch1,
		})
		assert.NoError(t, err)
	})

	t.Run("success: store proofs for epoch 0", func(t *testing.T) {
		validCurrentValidatorsProof := types.ValidatorsProof{
			BlockHeight: epoch0ActiveParticipantsHeight,
			Round:       0,
			BlockId:     validHeaderEpoch0.LastBlockId,
			Signatures: []*types.SignatureInfo{
				{
					SignatureBase64:     "ELKCYIpa7AVhaVVbmPDFwQ/tyj4ZKY6p6s37bBiz2U2E1wOw0mEpKqGiF6TQegoMfi6lY5MWh+IbuMudeH3fCA==",
					ValidatorAddressHex: "7991A95AD2C4906F847C9EDA58E965C88265C4BD",
					Timestamp:           mustParseTime(t, "2025-09-04T13:05:41.29844009Z"),
				},
			},
		}

		validNextBlockValidatorsProof := types.ValidatorsProof{
			BlockHeight: epoch0ActiveParticipantsHeight + 1,
			BlockId: &types.BlockID{
				PartSetHeaderTotal: 1,
				PartSetHeaderHash:  "435D1A0507B598612E089BD6AF037F3EC0B38A3076609C3A3B93E4B6F1F0B944",
			},
			Signatures: []*types.SignatureInfo{
				{
					SignatureBase64:     "2H5CoR2imb6X0f6MJgyYq2wIySfBI31jCpNqAGO1iusC79twlGnILRfgdKyh0sbyRMy4V3B+7+YVcSd6ESB6AA==",
					ValidatorAddressHex: "7991A95AD2C4906F847C9EDA58E965C88265C4BD",
					Timestamp:           mustParseTime(t, "2025-09-04T13:05:46.333070912Z"),
				},
			},
		}

		k, ctx := keeper.InferenceKeeper(t)
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		ctx = sdkCtx.WithChainID("gonka-mainnet")

		k.SetActiveParticipants(ctx, validActiveParticiapntsEpoch0)
		msgSrv := setupMsgServerWithKeeper(k)
		_, err := msgSrv.SubmitMissingParticipantsProofData(ctx, &types.MsgSubmitActiveParticipantsProofData{
			BlockHeight:                 epoch0ActiveParticipantsHeight,
			CurrentBlockValidatorsProof: &validCurrentValidatorsProof,
			NextBlockValidatorsProof:    &validNextBlockValidatorsProof,
			BlockProof:                  &validHeaderEpoch0,
			ProofOpts:                   nil,
		})
		assert.NoError(t, err)
	})

	t.Run("fail: set proof for epoch_1 BEFORE epoch_0", func(t *testing.T) {
		k, ctx := keeper.InferenceKeeper(t)
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		ctx = sdkCtx.WithChainID("gonka-mainnet")

		k.SetActiveParticipants(ctx, validCurrentActiveParticiapnts)

		msgSrv := setupMsgServerWithKeeper(k)
		_, err := msgSrv.SubmitMissingParticipantsProofData(ctx, &types.MsgSubmitActiveParticipantsProofData{
			BlockHeight:                 epoch1ActiveParticipantsHeight,
			EpochId:                     currentEpoch,
			CurrentBlockValidatorsProof: &validCurrentValidatorsProof,
			NextBlockValidatorsProof:    &validNextBlockValidatorsProof,
			BlockProof:                  &validHeaderEpoch1,
			ProofOpts:                   &validProofOpsEpoch1,
		})
		assert.ErrorContains(t, err, "participants for previous epoch not found")
	})

	t.Run("fail: wrong header", func(t *testing.T) {
		k, ctx := keeper.InferenceKeeper(t)
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		ctx = sdkCtx.WithChainID("gonka-mainnet")

		k.SetActiveParticipants(ctx, validActiveParticiapntsEpoch0)
		k.SetActiveParticipants(ctx, validCurrentActiveParticiapnts)

		invalidHeader := validHeaderEpoch1
		invalidHeader.AppHash = validHeaderEpoch0.AppHash

		validHeaderEpoch0.Height = epoch1ActiveParticipantsHeight
		msgSrv := setupMsgServerWithKeeper(k)
		_, err := msgSrv.SubmitMissingParticipantsProofData(ctx, &types.MsgSubmitActiveParticipantsProofData{
			BlockHeight:                 epoch1ActiveParticipantsHeight,
			EpochId:                     currentEpoch,
			CurrentBlockValidatorsProof: &validCurrentValidatorsProof,
			NextBlockValidatorsProof:    &validNextBlockValidatorsProof,
			BlockProof:                  &invalidHeader,
			ProofOpts:                   &validProofOpsEpoch1,
		})
		assert.ErrorContains(t, err, "failed to verify signature")
	})

	t.Run("fail: invalid merkle proof", func(t *testing.T) {
		k, ctx := keeper.InferenceKeeper(t)
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		ctx = sdkCtx.WithChainID("gonka-mainnet")

		k.SetActiveParticipants(ctx, validActiveParticiapntsEpoch0)
		k.SetActiveParticipants(ctx, validCurrentActiveParticiapnts)

		invalidProofOps := &types.ProofOps{
			BlockHeight: validProofOpsEpoch1.BlockHeight,
			Epoch:       validProofOpsEpoch1.Epoch,
			Ops: []types.ProofOp{
				validProofOpsEpoch1.Ops[1],
				validProofOpsEpoch1.Ops[0],
			},
		}

		msgSrv := setupMsgServerWithKeeper(k)
		_, err := msgSrv.SubmitMissingParticipantsProofData(ctx, &types.MsgSubmitActiveParticipantsProofData{
			BlockHeight:                 epoch1ActiveParticipantsHeight,
			EpochId:                     currentEpoch,
			CurrentBlockValidatorsProof: &validCurrentValidatorsProof,
			NextBlockValidatorsProof:    &validNextBlockValidatorsProof,
			BlockProof:                  &validHeaderEpoch1,
			ProofOpts:                   invalidProofOps,
		})
		assert.ErrorContains(t, err, "unexpected first proof op type")
	})
}

func mustDecodeHex(t *testing.T, s string) []byte {
	bz, err := hex.DecodeString(s)
	assert.NoError(t, err)
	return bz
}

func mustDecodeBase64(t *testing.T, s string) []byte {
	bz, err := base64.StdEncoding.DecodeString(s)
	assert.NoError(t, err)
	return bz
}

func mustParseTime(t *testing.T, s string) time.Time {
	tt, err := time.Parse(time.RFC3339, s)
	assert.NoError(t, err)
	return tt
}
