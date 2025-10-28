package calculations

import (
	"testing"

	"github.com/productscience/inference/x/inference/types"
	"github.com/stretchr/testify/require"
)

func TestZScoreCalculator(t *testing.T) {
	// Separately calculate values to confirm results
	equal := CalculateZScoreFromFPR(0.05, 95, 5)
	require.Equal(t, 0.0, equal)

	negative := CalculateZScoreFromFPR(0.05, 96, 4)
	require.InDelta(t, -0.458831, negative, 0.00001)

	positive := CalculateZScoreFromFPR(0.05, 94, 6)
	require.InDelta(t, 0.458831, positive, 0.00001)

	bigNegative := CalculateZScoreFromFPR(0.05, 960, 40)
	require.InDelta(t, -1.450953, bigNegative, 0.00001)

	bigPositive := CalculateZScoreFromFPR(0.05, 940, 60)
	require.InDelta(t, 1.450953, bigPositive, 0.00001)
}

func TestMeasurementsNeeded(t *testing.T) {
	tests := []struct {
		name string
		p    float64
		max  uint64
		want uint64
	}{
		{
			name: "5% false positive rate, max 100",
			p:    0.05,
			max:  100,
			want: 53,
		},
		{
			name: "10% false positive rate, max 100",
			p:    0.10,
			max:  100,
			want: 27,
		},
		{
			name: "1% false positive rate, max 300",
			p:    0.01,
			max:  300,
			want: 262,
		},
		{
			name: "1% false positive rate, max 100",
			p:    0.01,
			max:  100,
			want: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MeasurementsNeeded(tt.p, tt.max))
		})
	}
}

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		name        string
		params      *types.ValidationParams
		participant types.Participant
		wantStatus  types.ParticipantStatus
		wantReason  ParticipantStatusReason
	}{
		{
			name:        "nil validation parameters returns active",
			params:      nil,
			participant: types.Participant{},
			wantStatus:  types.ParticipantStatus_ACTIVE,
			wantReason:  NoReason,
		},
		{
			name: "consecutive failures returns invalid",
			params: &types.ValidationParams{
				FalsePositiveRate: types.DecimalFromFloat(0.05),
			},
			participant: types.Participant{
				ConsecutiveInvalidInferences: 20,
			},
			wantStatus: types.ParticipantStatus_INVALID,
			wantReason: ConsecutiveFailures,
		},
		{
			name: "ramping up returns ramping",
			params: &types.ValidationParams{
				FalsePositiveRate:     types.DecimalFromFloat(0.05),
				MinRampUpMeasurements: 100,
			},
			participant: types.Participant{
				CurrentEpochStats: &types.CurrentEpochStats{
					InferenceCount: 50,
				},
				EpochsCompleted: 0,
			},
			wantStatus: types.ParticipantStatus_RAMPING,
			wantReason: Ramping,
		},
		{
			name: "statistical invalidations returns invalid",
			params: &types.ValidationParams{
				FalsePositiveRate: types.DecimalFromFloat(0.05),
			},
			participant: types.Participant{
				CurrentEpochStats: &types.CurrentEpochStats{
					ValidatedInferences:   80,
					InvalidatedInferences: 20,
				},
			},
			wantStatus: types.ParticipantStatus_INVALID,
			wantReason: StatisticalInvalidations,
		},
		{
			name: "normal operation returns active",
			params: &types.ValidationParams{
				FalsePositiveRate: types.DecimalFromFloat(0.05),
			},
			participant: types.Participant{
				CurrentEpochStats: &types.CurrentEpochStats{
					ValidatedInferences:   95,
					InvalidatedInferences: 5,
				},
			},
			wantStatus: types.ParticipantStatus_ACTIVE,
			wantReason: NoReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := ComputeStatus(tt.params, tt.participant)
			require.Equal(t, tt.wantStatus, status)
			require.Equal(t, tt.wantReason, reason)
		})
	}
}
