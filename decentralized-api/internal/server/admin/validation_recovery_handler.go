package admin

import (
	"decentralized-api/cosmosclient"
	"decentralized-api/logging"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/productscience/inference/api/inference/inference"
	"github.com/productscience/inference/x/inference/types"
)

type ClaimRewardRecoverRequest struct {
	Seed       *int64 `json:"seed,omitempty"` // Optional: if not provided, uses stored seed
	ForceClaim bool   `json:"force_claim"`    // Force claim even if already claimed
}

type ClaimRewardRecoverResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	EpochIndex        uint64 `json:"epoch_index"`
	Seed              int64  `json:"seed"`
	MissedValidations int    `json:"missed_validations"`
	AlreadyClaimed    bool   `json:"already_claimed"`
	ClaimExecuted     bool   `json:"claim_executed"`
}

func (s *Server) postClaimRewardRecover(ctx echo.Context) error {
	var req ClaimRewardRecoverRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Always use the previous epoch (only epoch we can recover)
	previousSeed := s.configManager.GetPreviousSeed()
	epochIndex := previousSeed.EpochIndex

	// Determine the seed to use
	var seedValue int64
	if req.Seed != nil {
		// Custom seed provided
		seedValue = *req.Seed
	} else {
		// Use stored seed
		seedValue = previousSeed.Seed
	}

	// Check if seed is valid
	if seedValue == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "No valid seed available for previous epoch "+strconv.FormatUint(epochIndex, 10))
	}

	// Check if already claimed
	alreadyClaimed := s.configManager.IsPreviousSeedClaimed()
	if alreadyClaimed && !req.ForceClaim {
		return ctx.JSON(http.StatusOK, ClaimRewardRecoverResponse{
			Success:           false,
			Message:           "Rewards already claimed for this epoch. Use force_claim=true to override.",
			EpochIndex:        epochIndex,
			Seed:              seedValue,
			MissedValidations: 0,
			AlreadyClaimed:    true,
			ClaimExecuted:     false,
		})
	}

	logging.Info("Starting manual validation recovery", types.Validation,
		"epochIndex", epochIndex,
		"seed", seedValue,
		"alreadyClaimed", alreadyClaimed,
		"forceClaim", req.ForceClaim)

	// Detect missed validations
	missedInferences, err := s.validator.DetectMissedValidations(epochIndex, seedValue)
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
			Seed:       seedValue,
			EpochIndex: epochIndex,
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
		Seed:              seedValue,
		MissedValidations: missedCount,
		AlreadyClaimed:    alreadyClaimed,
		ClaimExecuted:     claimExecuted,
	})
}
