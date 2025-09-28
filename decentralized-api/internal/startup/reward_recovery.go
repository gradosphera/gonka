package startup

import (
	"context"
	"decentralized-api/apiconfig"
	"decentralized-api/chainphase"
	"decentralized-api/cosmosclient"
	"decentralized-api/internal/validation"
	"decentralized-api/logging"
	"time"

	"github.com/productscience/inference/api/inference/inference"
	"github.com/productscience/inference/x/inference/types"
)

const waitTimeBlocksFromLaunch = 60
const waitBetweenAttempts = 100

type RewardRecoveryChecker struct {
	launchBlockHeight       int64
	lastRecoveryBlockHeight int64
	phaseTracker            *chainphase.ChainPhaseTracker
	recorder                *cosmosclient.InferenceCosmosClient
	validator               *validation.InferenceValidator
	configManager           *apiconfig.ConfigManager
}

func (c *RewardRecoveryChecker) RecoverIfNeeded(
	currentBlockHeight int64,
) {
	if currentBlockHeight < (c.launchBlockHeight + waitTimeBlocksFromLaunch) {
		logging.Debug("Waiting for launch", types.Claims,
			"currentBlockHeight", currentBlockHeight,
			"launchBlockHeight", c.launchBlockHeight)
		return
	}

	if currentBlockHeight < (c.lastRecoveryBlockHeight + waitBetweenAttempts) {
		logging.Debug("Waiting for last recovery", types.Claims,
			"currentBlockHeight", currentBlockHeight,
			"lastRecoveryBlockHeight", c.lastRecoveryBlockHeight)
		return
	}

	latestEpoch := c.phaseTracker.GetCurrentEpochState().LatestEpoch
	inferenceValidationCutoff := latestEpoch.InferenceValidationCutoff()
	if currentBlockHeight > inferenceValidationCutoff {
		logging.Debug("Inference validation cutoff reached", types.Claims,
			"currentBlockHeight", currentBlockHeight,
			"inferenceValidationCutoff", inferenceValidationCutoff)
		return
	}

	if latestEpoch.GetCurrentPhase(currentBlockHeight) != types.InferencePhase {
		logging.Debug("Not in inference phase", types.Claims,
			"currentBlockHeight", currentBlockHeight,
			"latestEpoch", latestEpoch)
		return
	}

	c.lastRecoveryBlockHeight = currentBlockHeight
	go func() {
		c.AutoRewardRecovery()
	}()
}

// AutoRewardRecovery checks for unclaimed settle amounts and attempts to recover rewards on startup
func (c *RewardRecoveryChecker) AutoRewardRecovery() {
	logging.Info("Starting automatic reward recovery check", types.Claims)

	// Get participant address
	address := c.recorder.GetAddress()
	if address == "" {
		logging.Error("Cannot perform reward recovery: no participant address", types.Claims)
		return
	}

	// Query for settle amount
	queryClient := c.recorder.NewInferenceQueryClient()
	ctx, cancel := context.WithTimeout(c.recorder.GetContext(), 30*time.Second)
	defer cancel()

	settleAmountResp, err := queryClient.SettleAmount(ctx, &types.QueryGetSettleAmountRequest{
		Participant: address,
	})
	if err != nil {
		// This is expected if no settle amount exists
		logging.Debug("No settle amount found for participant", types.Claims, "address", address, "error", err)
		return
	}

	if settleAmountResp == nil {
		logging.Debug("No settle amount data available", types.Claims, "address", address)
		return
	}

	settleAmount := settleAmountResp.SettleAmount
	totalAmount := settleAmount.RewardCoins + settleAmount.WorkCoins
	logging.Info("Found settle amount for participant", types.Claims,
		"address", address,
		"rewardCoins", settleAmount.RewardCoins,
		"workCoins", settleAmount.WorkCoins,
		"totalAmount", totalAmount,
		"epochIndex", settleAmount.EpochIndex)

	// Check if we have unclaimed rewards (totalAmount > 0 indicates pending rewards)
	if totalAmount <= 0 {
		logging.Info("No unclaimed rewards found", types.Claims, "address", address, "totalAmount", totalAmount)
		return
	}

	// Get the previous seed for this epoch
	previousSeed := c.configManager.GetPreviousSeed()

	// Check if the settle amount epoch matches our stored epoch
	if previousSeed.EpochIndex != settleAmount.EpochIndex {
		logging.Warn("Settle amount epoch doesn't match stored previous seed epoch", types.Claims,
			"settleAmountEpoch", settleAmount.EpochIndex,
			"storedSeedEpoch", previousSeed.EpochIndex,
			"address", address)

		// We could still try with the settle amount epoch, but it's risky
		// For now, let's be conservative and skip
		return
	}

	// Check if we have a valid seed
	if previousSeed.Seed == 0 {
		logging.Warn("No valid seed available for reward recovery", types.Claims,
			"epochIndex", settleAmount.EpochIndex,
			"address", address)
		return
	}

	logging.Info("Attempting automatic reward recovery", types.Claims,
		"epochIndex", settleAmount.EpochIndex,
		"seed", previousSeed.Seed,
		"totalAmount", totalAmount,
		"address", address)

	// Perform validation recovery using the same logic as the admin endpoint
	missedInferences, err := c.validator.DetectMissedValidations(previousSeed.EpochIndex, previousSeed.Seed)
	if err != nil {
		logging.Error("Failed to detect missed validations during startup", types.Claims,
			"epochIndex", settleAmount.EpochIndex,
			"error", err)
		return
	}

	missedCount := len(missedInferences)
	logging.Info("Startup recovery detected missed validations", types.Claims,
		"epochIndex", settleAmount.EpochIndex,
		"missedCount", missedCount,
		"address", address)

	// Execute recovery validations if any were missed
	if missedCount > 0 {
		recoveredCount, err := c.validator.ExecuteRecoveryValidations(missedInferences)
		if err != nil {
			logging.Error("Failed to execute recovery validations during startup", types.Claims,
				"epochIndex", settleAmount.EpochIndex,
				"missedCount", missedCount,
				"error", err)
			return
		}

		logging.Info("Startup recovery validations completed", types.Claims,
			"epochIndex", settleAmount.EpochIndex,
			"recoveredCount", recoveredCount,
			"missedCount", missedCount,
			"address", address)

		// Wait for validations to be recorded on-chain
		if recoveredCount > 0 {
			logging.Info("Waiting for startup recovery validations to be recorded on-chain", types.Claims,
				"epochIndex", settleAmount.EpochIndex,
				"recoveredCount", recoveredCount)
			c.validator.WaitForValidationsToBeRecorded()
		}
	}

	// Attempt to claim rewards
	err = c.recorder.ClaimRewards(&inference.MsgClaimRewards{
		Seed:       previousSeed.Seed,
		EpochIndex: previousSeed.EpochIndex,
	})
	if err != nil {
		logging.Error("Failed to claim rewards during startup recovery", types.Claims,
			"epochIndex", settleAmount.EpochIndex,
			"error", err)
		return
	}

	// Mark as claimed to prevent duplicate attempts
	err = c.configManager.MarkPreviousSeedClaimed()
	if err != nil {
		logging.Error("Failed to mark seed as claimed after successful recovery", types.Claims,
			"epochIndex", settleAmount.EpochIndex,
			"error", err)
	}

	logging.Info("Automatic reward recovery completed successfully", types.Claims,
		"epochIndex", settleAmount.EpochIndex,
		"address", address)
}
