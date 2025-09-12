package admin

import (
	"context"
	"decentralized-api/cosmosclient"
	"decentralized-api/internal/utils"
	"decentralized-api/logging"
	"encoding/base64"
	"errors"
	"fmt"
	rpcclient "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/labstack/echo/v4"
	"github.com/productscience/inference/x/inference/types"
	"net/http"
	"time"
)

type populateDataRequest struct {
	ArchiveNodeRpcEndpoint string `json:"archiveNodeRpcEndpoint" validate:"required"`
	StartFromEpoch         uint64 `json:"startFromEpoch"`
}

func (s *Server) populateMissingProofs(ctx echo.Context) error {
	var req populateDataRequest

	if err := ctx.Bind(&req); err != nil {
		return err
	}

	if req.ArchiveNodeRpcEndpoint == "" {
		return ctx.JSON(http.StatusBadRequest, "archiveNodeRpcEndpoint is required")
	}

	return fillDataForUpgrade(ctx.Request().Context(), s.recorder, req.ArchiveNodeRpcEndpoint, req.StartFromEpoch)
}

func fillDataForUpgrade(
	ctx context.Context,
	transactionRecorder cosmosclient.CosmosMessageClient,
	archiveNodeEndpoint string, startFromEpoch uint64) error {
	archiveClient, err := rpcclient.New(archiveNodeEndpoint, "/websocket")
	if err != nil {
		logging.Error("FillDataForUpgrade: failed create rpc client for archive node", types.System, "err", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	queryClient := transactionRecorder.NewInferenceQueryClient()
	currEpoch, err := queryClient.GetCurrentEpoch(ctx, &types.QueryGetCurrentEpochRequest{})
	if err != nil {
		logging.Error("FillDataForUpgrade: Failed to get current epoch", types.System, "error", err)
		return err
	}
	logging.Info("FillDataForUpgrade: Current epoch resolved.", types.System, "epoch", currEpoch.Epoch)
	currentEpochId := currEpoch.Epoch

	if startFromEpoch > currentEpochId+1 {
		return errors.New("startFromEpoch cannot be greater than current epoch")
	}

	for epochId := startFromEpoch; epochId <= currentEpochId+1; epochId++ {
		dataKey := types.ActiveParticipantsFullKey(epochId)
		result, err := cosmosclient.QueryByKey(archiveClient, "inference", dataKey)
		if err != nil {
			logging.Error("FillDataForUpgrade: Failed to query active participants", types.Participants, "epoch_id", epochId, "err", err)
			return err
		}

		interfaceRegistry := codectypes.NewInterfaceRegistry()
		types.RegisterInterfaces(interfaceRegistry)
		cdc := codec.NewProtoCodec(interfaceRegistry)

		var activeParticipants types.ActiveParticipants
		if err := cdc.Unmarshal(result.Response.Value, &activeParticipants); err != nil {
			logging.Error("FillDataForUpgrade: Failed to unmarshal active participants. Req 1", types.Participants, "error", err)
			return err
		}

		proofBlockHeight := activeParticipants.CreatedAtBlockHeight + 1
		nextBlockHeight := activeParticipants.CreatedAtBlockHeight + 2
		proofBlock, err := archiveClient.Block(ctx, &proofBlockHeight)
		if err != nil {
			logging.Error("FillDataForUpgrade: failed get archive block", types.System, "height", proofBlockHeight, "err", err)
			return err
		}

		nextBlock, err := archiveClient.Block(ctx, &nextBlockHeight)
		if err != nil {
			logging.Error("FillDataForUpgrade: failed get archive block", types.System, "height", nextBlockHeight, "err", err)
			return err
		}

		var proofOps *types.ProofOps
		if epochId != 0 {
			proofOps, err = utils.GetParticipantsProof(archiveClient, epochId, activeParticipants.CreatedAtBlockHeight)
			if err != nil {
				logging.Error("FillDataForUpgrade: failed get proof ops", types.System, "height", activeParticipants.CreatedAtBlockHeight, "err", err)
				return err
			}
		}

		proofBlockId := &types.BlockID{
			Hash:               proofBlock.Block.Header.LastBlockID.Hash.String(),
			PartSetHeaderTotal: int64(proofBlock.Block.Header.LastBlockID.PartSetHeader.Total),
			PartSetHeaderHash:  proofBlock.Block.Header.LastBlockID.PartSetHeader.Hash.String(),
		}

		fmt.Printf("FillDataForUpgrade: proofBlockId: %+v\n", proofBlockId)
		header := types.BlockHeaderFull{
			Version:            int64(proofBlock.Block.Version.Block),
			ChainId:            proofBlock.Block.ChainID,
			Height:             proofBlock.Block.Height,
			Timestamp:          proofBlock.Block.Time,
			LastBlockId:        proofBlockId,
			LastCommitHash:     proofBlock.Block.Header.LastCommitHash,
			DataHash:           proofBlock.Block.Header.DataHash,
			ValidatorsHash:     proofBlock.Block.Header.ValidatorsHash,
			NextValidatorsHash: proofBlock.Block.Header.NextValidatorsHash,
			ConsensusHash:      proofBlock.Block.Header.ConsensusHash,
			AppHash:            proofBlock.Block.Header.AppHash,
			LastResultsHash:    proofBlock.Block.Header.LastResultsHash,
			EvidenceHash:       proofBlock.Block.Header.EvidenceHash,
			ProposerAddress:    proofBlock.Block.Header.ProposerAddress,
		}

		fmt.Printf("FillDataForUpgrade: header: %+v\n", header)

		currentValidatorsProof := createValidatorsProofFromBlock(proofBlockId, proofBlock.Block.LastCommit)
		fmt.Printf("FillDataForUpgrade: currentValidatorsProof: %+v\n", currentValidatorsProof)

		nextValidatorsProof := createValidatorsProofFromBlock(&types.BlockID{
			Hash:               nextBlock.Block.Header.LastBlockID.Hash.String(),
			PartSetHeaderTotal: int64(nextBlock.Block.Header.LastBlockID.PartSetHeader.Total),
			PartSetHeaderHash:  nextBlock.Block.Header.LastBlockID.PartSetHeader.Hash.String(),
		}, nextBlock.Block.LastCommit)
		fmt.Printf("FillDataForUpgrade: nextValidatorsProof: %+v\n", nextValidatorsProof)

		tx := &types.MsgSubmitActiveParticipantsProofData{
			BlockHeight:                 uint64(activeParticipants.CreatedAtBlockHeight),
			EpochId:                     epochId,
			CurrentBlockValidatorsProof: &currentValidatorsProof,
			NextBlockValidatorsProof:    &nextValidatorsProof,
			BlockProof:                  &header,
			ProofOpts:                   proofOps,
		}

		if err := transactionRecorder.SubmitMissingProofs(tx); err != nil {
			logging.Error("FillDataForUpgrade: Failed to submit proof", types.System, "err", err)
			return err
		}
	}
	return nil
}

func createValidatorsProofFromBlock(blockId *types.BlockID, commit *coretypes.Commit) types.ValidatorsProof {
	signatures := make([]*types.SignatureInfo, len(commit.Signatures))
	for i, sign := range commit.Signatures {
		signatures[i] = &types.SignatureInfo{
			SignatureBase64:     base64.StdEncoding.EncodeToString(sign.Signature),
			ValidatorAddressHex: sign.ValidatorAddress.String(),
			Timestamp:           sign.Timestamp,
		}
	}

	return types.ValidatorsProof{
		BlockHeight: commit.Height,
		Round:       int64(commit.Round),
		BlockId:     blockId,
		Signatures:  signatures,
	}
}
