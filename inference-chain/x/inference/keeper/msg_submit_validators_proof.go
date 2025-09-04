package keeper

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/cometbft/cometbft/proto/tendermint/version"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gonka-ai/gonka-utils/go/utils"
	"github.com/productscience/common"
	"github.com/productscience/inference/x/inference/types"
	"strings"
)

func (s msgServer) SubmitParticipantsProof(goCtx context.Context, msg *types.MsgSubmitParticipantsProof) (*types.MsgSubmitParticipantsProofResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.BlockHeight == 0 {
		return nil, errors.New("block height must be set")
	}

	if msg.ValidatorsProof != nil {
		if err := s.Keeper.SetValidatorsProof(ctx, *msg.ValidatorsProof); err != nil {
			return nil, err
		}
	}

	if msg.ProofOpts != nil {
		if err := s.Keeper.SetActiveParticipantsProof(ctx, *msg.ProofOpts, msg.BlockHeight); err != nil {
			return nil, err
		}
	}
	return &types.MsgSubmitParticipantsProofResponse{}, nil
}

func (s msgServer) SubmitMissingParticipantsProofData(ctx context.Context, msg *types.MsgSubmitActiveParticipantsProofData) (*types.MsgSubmitActiveParticipantsProofDataResponse, error) {
	if msg.BlockHeight == 0 {
		return nil, errors.New("block height must be set")
	}

	if msg.CurrentBlockValidatorsProof == nil ||
		msg.NextBlockValidatorsProof == nil ||
		msg.BlockProof == nil {
		return nil, errors.New("one of the mandatory proofs missing")
	}

	if msg.EpochId != 0 && msg.ProofOpts == nil {
		return nil, errors.New("merkle proof is mandatory for epoch_id > 0")
	}

	// 1. make sure current block validators proof, next block validators proof and next block header  are really formed from N and N+1 blocks
	if int64(msg.BlockHeight) != msg.BlockProof.Height-1 || msg.CurrentBlockValidatorsProof.BlockHeight != msg.BlockProof.Height-1 {
		return nil, errors.New("invalid height by block proof")
	}

	if strings.ToUpper(msg.CurrentBlockValidatorsProof.BlockId.Hash) != strings.ToUpper(msg.BlockProof.LastBlockId.Hash) {
		return nil, errors.New("invalid hash by block proof")
	}

	// 2. make sure active participants set exists for given epoch and given proofs data os for reight block
	currentParticipants, found := s.Keeper.GetActiveParticipants(ctx, msg.EpochId)
	if !found {
		return nil, errors.New("participants for given epoch not found")
	}

	if currentParticipants.CreatedAtBlockHeight != int64(msg.BlockHeight) ||
		currentParticipants.CreatedAtBlockHeight != msg.CurrentBlockValidatorsProof.BlockHeight {
		return nil, errors.New("proofs block height do not match participants block height")
	}

	var prevParticipants types.ActiveParticipants
	if msg.EpochId == 0 {
		prevParticipants, found = s.Keeper.GetActiveParticipants(ctx, msg.EpochId)
	} else {
		epoch := msg.EpochId - 1
		prevParticipants, found = s.Keeper.GetActiveParticipants(ctx, epoch)
	}
	if !found {
		return nil, errors.New("participants for previous epoch not found")
	}

	participantsData := make(map[string]string)

	for _, participant := range prevParticipants.Participants {
		addrHex, err := common.ConsensusKeyToConsensusAddress(participant.ValidatorKey)
		if err != nil {
			return nil, err
		}
		participantsData[strings.ToUpper(addrHex)] = participant.ValidatorKey
	}

	if err := verifyGivenProofs(msg, participantsData); err != nil {
		return nil, err
	}

	// success, store proofs
	commits := make([]*types.CommitInfo, len(msg.CurrentBlockValidatorsProof.Signatures))
	for i, sign := range msg.CurrentBlockValidatorsProof.Signatures {
		pubKey := participantsData[sign.ValidatorAddressHex]
		commits[i] = &types.CommitInfo{
			ValidatorAddress: sign.ValidatorAddressHex,
			ValidatorPubKey:  pubKey,
		}
	}

	if err := s.Keeper.SetBlockProof(ctx, types.BlockProof{
		CreatedAtBlockHeight: int64(msg.BlockHeight),
		AppHashHex:           hex.EncodeToString(msg.BlockProof.AppHash),
		Commits:              commits,
	}); err != nil {
		return nil, err
	}

	if err := s.Keeper.SetValidatorsProof(ctx, *msg.CurrentBlockValidatorsProof); err != nil {
		return nil, err
	}

	if msg.ProofOpts != nil {
		if err := s.Keeper.SetActiveParticipantsProof(ctx, *msg.ProofOpts, msg.BlockHeight); err != nil {
			return nil, err
		}
	}
	return &types.MsgSubmitActiveParticipantsProofDataResponse{}, nil
}

func verifyGivenProofs(msg *types.MsgSubmitActiveParticipantsProofData, participantsData map[string]string) error {
	for _, sign := range msg.CurrentBlockValidatorsProof.Signatures {
		if _, found := participantsData[strings.ToUpper(sign.ValidatorAddressHex)]; !found {
			return errors.New("validator address not found in previous participants")
		}
	}

	for _, sign := range msg.NextBlockValidatorsProof.Signatures {
		if _, found := participantsData[strings.ToUpper(sign.ValidatorAddressHex)]; !found {
			return errors.New("validator address not found in previous participants")
		}
	}

	// 2. verify current block signatures
	currentProof := common.ToContractsValidatorsProof(msg.CurrentBlockValidatorsProof)
	err := utils.VerifySignatures(*currentProof, msg.BlockProof.ChainId, participantsData)
	if err != nil {
		return err
	}

	// 3. verify app hash: validators in next block must sign header of current block
	// hash of header == hash of block id
	lastBlockIDhashBytes, err := hex.DecodeString(msg.BlockProof.LastBlockId.Hash)
	if err != nil {
		return err
	}

	partSetHeaderHash, err := hex.DecodeString(msg.BlockProof.LastBlockId.PartSetHeaderHash)
	if err != nil {
		return err
	}

	header := cmttypes.Header{
		Version: version.Consensus{
			Block: uint64(msg.BlockProof.Version),
		},
		ChainID: msg.BlockProof.ChainId,
		Height:  msg.BlockProof.Height,
		Time:    msg.BlockProof.Timestamp,
		LastBlockID: cmttypes.BlockID{
			Hash: lastBlockIDhashBytes,
			PartSetHeader: cmttypes.PartSetHeader{
				Total: uint32(msg.BlockProof.LastBlockId.PartSetHeaderTotal),
				Hash:  partSetHeaderHash,
			},
		},
		LastCommitHash:     msg.BlockProof.LastCommitHash,
		DataHash:           msg.BlockProof.DataHash,
		ValidatorsHash:     msg.BlockProof.ValidatorsHash,
		NextValidatorsHash: msg.BlockProof.NextValidatorsHash,
		ConsensusHash:      msg.BlockProof.ConsensusHash,
		AppHash:            msg.BlockProof.AppHash,
		LastResultsHash:    msg.BlockProof.LastResultsHash,
		EvidenceHash:       msg.BlockProof.EvidenceHash,
		ProposerAddress:    msg.BlockProof.ProposerAddress,
	}

	nextProof := common.ToContractsValidatorsProof(msg.NextBlockValidatorsProof)
	nextProof.BlockId.Hash = header.Hash().String() // use calculated hash as block id
	return utils.VerifySignatures(*nextProof, msg.BlockProof.ChainId, participantsData)
}
