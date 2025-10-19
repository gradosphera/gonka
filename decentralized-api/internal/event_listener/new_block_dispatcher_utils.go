package event_listener

import (
	"decentralized-api/internal/event_listener/chainevents"
	"decentralized-api/logging"
	"encoding/base64"
	"github.com/productscience/inference/x/inference/types"
	"strconv"
)

func fillValidatorsProof(lastCommit chainevents.LastCommit) (*types.ValidatorsProof, error) {
	height, err := strconv.ParseInt(lastCommit.Height, 10, 64)
	if err != nil {
		logging.Error("Failed to parse block height to int", types.ParticipantsVerification, "height", lastCommit.Height, "error", err)
		return nil, err
	}

	proof := &types.ValidatorsProof{
		BlockHeight: height,
		Round:       int64(lastCommit.Round),
		BlockId: &types.BlockID{
			Hash:               lastCommit.BlockId.Hash.String(),
			PartSetHeaderTotal: int64(lastCommit.BlockId.PartSetHeader.Total),
			PartSetHeaderHash:  lastCommit.BlockId.PartSetHeader.Hash.String(),
		},
		Signatures: make([]*types.SignatureInfo, 0),
	}

	for _, sign := range lastCommit.Signatures {
		encodedSign := base64.StdEncoding.EncodeToString(sign.Signature)

		logging.Info("Preparing signature to send", types.ParticipantsVerification,
			"signature_ts", sign.Timestamp,
			"signature", encodedSign,
			"height", height,
			"validator_address", sign.ValidatorAddress.String())

		proof.Signatures = append(proof.Signatures, &types.SignatureInfo{
			SignatureBase64:     encodedSign,
			ValidatorAddressHex: sign.ValidatorAddress.String(),
			Timestamp:           sign.Timestamp,
		})
	}
	return proof, nil
}
