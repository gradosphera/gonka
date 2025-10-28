package calculations

import (
	"math"

	"github.com/productscience/inference/x/inference/types"
)

type ParticipantStatusReason string

const (
	// ConsecutiveFailures indicates the participant has too many consecutive failures
	ConsecutiveFailures ParticipantStatusReason = "consecutive_failures"
	// Ramping indicates the participant is in ramp-up phase
	Ramping ParticipantStatusReason = "ramping"
	// StatisticalInvalidations indicates the participant has statistically significant invalidations
	StatisticalInvalidations ParticipantStatusReason = "statistical_invalidations"
	// NoReason indicates no specific reason for the status
	NoReason ParticipantStatusReason = ""
)

func ComputeStatus(validationParameters *types.ValidationParams, participant types.Participant) (status types.ParticipantStatus, reason ParticipantStatusReason) {
	// Genesis only (for tests)
	if validationParameters == nil || validationParameters.FalsePositiveRate == nil {
		return types.ParticipantStatus_ACTIVE, NoReason
	}
	// If we have consecutive failures with a likelihood of less than 1 in a million times, we're assuming bad
	falsePositiveRate := validationParameters.FalsePositiveRate.ToFloat()
	if probabilityOfConsecutiveFailures(falsePositiveRate, participant.ConsecutiveInvalidInferences) < 0.000001 {
		return types.ParticipantStatus_INVALID, ConsecutiveFailures
	}

	// Solely for Genesis testing/automation
	if participant.CurrentEpochStats == nil {
		participant.CurrentEpochStats = &types.CurrentEpochStats{}
	}

	zScore := CalculateZScoreFromFPR(falsePositiveRate, participant.CurrentEpochStats.ValidatedInferences, participant.CurrentEpochStats.InvalidatedInferences)
	needed := MeasurementsNeeded(falsePositiveRate, uint64(validationParameters.MinRampUpMeasurements))
	if participant.CurrentEpochStats.InferenceCount < needed && participant.EpochsCompleted < 1 {
		return types.ParticipantStatus_RAMPING, Ramping
	}

	if zScore > 1 {
		return types.ParticipantStatus_INVALID, StatisticalInvalidations
	}
	return types.ParticipantStatus_ACTIVE, NoReason
}

// CalculateZScoreFromFPR - Positive values mean the failure rate is HIGHER than expected, thus bad
func CalculateZScoreFromFPR(expectedFailureRate float64, valid uint64, invalid uint64) float64 {
	total := valid + invalid
	if total == 0 {
		return 0 // avoid division by zero; with zero observations we will be in RAMPING anyway
	}
	observedFailureRate := float64(invalid) / float64(total)

	// Calculate the variance using the binomial distribution formula
	variance := expectedFailureRate * (1 - expectedFailureRate) / float64(total)
	if variance == 0 {
		return 0
	}

	// Calculate the standard deviation
	stdDev := math.Sqrt(variance)

	// Calculate the z-score (how many standard deviations the observed failure rate is from the expected failure rate)
	zScore := (observedFailureRate - expectedFailureRate) / stdDev

	return zScore
}

// MeasurementsNeeded calculates the number of measurements required
// for a single failure to be within one standard deviation of the expected distribution
func MeasurementsNeeded(p float64, max uint64) uint64 {
	if p <= 0 || p >= 1 {
		panic("Probability p must be between 0 and 1, exclusive")
	}

	// This value is derived from solving the inequality: |1 - np| <= sqrt(np(1 - p))
	// Which leads to the quadratic inequality: y^2 - 3y + 1 >= 0, where y = np
	// The solution to this inequality is np >= (3 + sqrt(5)) / 2
	requiredValue := (3 + math.Sqrt(5)) / 2

	// Calculate the number of measurements
	n := requiredValue / p

	// Round up to the nearest whole number since we need an integer count of measurements
	needed := uint64(math.Ceil(n))
	if needed > max {
		return max
	}
	return needed
}

// probabilityOfConsecutiveFailures returns P(F^N|G) = x^N
func probabilityOfConsecutiveFailures(expectedFailureRate float64, consecutiveFailures int64) float64 {
	if expectedFailureRate < 0 || expectedFailureRate > 1 {
		panic("expectedFailureRate must be between 0 and 1")
	}
	if consecutiveFailures < 0 {
		panic("consecutiveFailures must be non-negative")
	}

	return math.Pow(expectedFailureRate, float64(consecutiveFailures))
}
