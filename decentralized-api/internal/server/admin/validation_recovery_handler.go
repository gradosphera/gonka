package admin

import (
	"decentralized-api/apiconfig"
	"decentralized-api/cosmosclient"
	"decentralized-api/logging"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/productscience/inference/api/inference/inference"
	"github.com/productscience/inference/x/inference/types"
)

type ClaimRewardRecoverRequest struct {
	EpochIndex *uint64 `json:"epoch_index,omitempty"` // Optional: if not provided, uses previous epoch
	ForceClaim bool    `json:"force_claim"`           // Force claim even if already claimed
}

type ClaimRewardRecoverResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	EpochIndex        uint64 `json:"epoch_index"`
	MissedValidations int    `json:"missed_validations"`
	AlreadyClaimed    bool   `json:"already_claimed"`
	ClaimExecuted     bool   `json:"claim_executed"`
}

func (s *Server) postClaimRewardRecover(ctx echo.Context) error {
	var req ClaimRewardRecoverRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Determine which epoch to recover
	var epochIndex uint64
	var seed apiconfig.SeedInfo

	if req.EpochIndex != nil {
		// Specific epoch requested
		epochIndex = *req.EpochIndex
		// For now, we can only recover the previous epoch (where we have the seed)
		previousSeed := s.configManager.GetPreviousSeed()
		if previousSeed.EpochIndex != epochIndex {
			return echo.NewHTTPError(http.StatusBadRequest,
				"Can only recover previous epoch. Current previous epoch: "+strconv.FormatUint(previousSeed.EpochIndex, 10))
		}
		seed = previousSeed
	} else {
		// Default to previous epoch
		seed = s.configManager.GetPreviousSeed()
		epochIndex = seed.EpochIndex
	}

	// Check if seed is valid
	if seed.Seed == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "No valid seed available for epoch "+strconv.FormatUint(epochIndex, 10))
	}

	// Check if already claimed
	alreadyClaimed := s.configManager.IsPreviousSeedClaimed()
	if alreadyClaimed && !req.ForceClaim {
		return ctx.JSON(http.StatusOK, ClaimRewardRecoverResponse{
			Success:           false,
			Message:           "Rewards already claimed for this epoch. Use force_claim=true to override.",
			EpochIndex:        epochIndex,
			MissedValidations: 0,
			AlreadyClaimed:    true,
			ClaimExecuted:     false,
		})
	}

	logging.Info("Starting manual validation recovery", types.Validation,
		"epochIndex", epochIndex,
		"seed", seed.Seed,
		"alreadyClaimed", alreadyClaimed,
		"forceClaim", req.ForceClaim)

	// Detect missed validations
	missedInferences, err := s.validator.DetectMissedValidations(epochIndex, seed.Seed)
	if err != nil {
		logging.Error("Failed to detect missed validations", types.Validation, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to detect missed validations: "+err.Error())
	}

	missedCount := len(missedInferences)
	logging.Info("Manual recovery detected missed validations", types.Validation,
		"epochIndex", epochIndex,
		"missedCount", missedCount)

	// Execute recovery validations
	if missedCount > 0 {
		s.validator.ExecuteRecoveryValidations(missedInferences)
		logging.Info("Manual recovery validations completed", types.Validation,
			"epochIndex", epochIndex,
			"recoveredCount", missedCount)
	}

	// Claim rewards if not already claimed or if forced
	claimExecuted := false
	if !alreadyClaimed || req.ForceClaim {
		// Cast to concrete type for RequestMoney
		concreteRecorder := s.recorder.(*cosmosclient.InferenceCosmosClient)
		err := concreteRecorder.ClaimRewards(&inference.MsgClaimRewards{
			Seed:       seed.Seed,
			EpochIndex: seed.EpochIndex,
		})
		if err != nil {
			logging.Error("Failed to claim rewards in manual recovery", types.Claims, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to claim rewards: "+err.Error())
		}

		// Mark as claimed
		err = s.configManager.MarkPreviousSeedClaimed()
		if err != nil {
			logging.Error("Failed to mark seed as claimed", types.Claims, "error", err)
		}

		claimExecuted = true
		logging.Info("Manual recovery claim executed", types.Claims, "epochIndex", epochIndex)
	}

	return ctx.JSON(http.StatusOK, ClaimRewardRecoverResponse{
		Success:           true,
		Message:           "Manual claim reward recovery completed successfully",
		EpochIndex:        epochIndex,
		MissedValidations: missedCount,
		AlreadyClaimed:    alreadyClaimed,
		ClaimExecuted:     claimExecuted,
	})
}
