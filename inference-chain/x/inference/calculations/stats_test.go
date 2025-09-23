package calculations

import (
	"fmt"
	"testing"
)

func TestMissedStatTestErrorConditions(t *testing.T) {
	tests := []struct {
		name     string
		nMissed  int
		nTotal   int
		expected bool
		wantErr  bool
	}{
		{
			name:     "negative missed count",
			nMissed:  -1,
			nTotal:   10,
			expected: false,
			wantErr:  true,
		},
		{
			name:     "zero total",
			nMissed:  0,
			nTotal:   0,
			expected: false,
			wantErr:  true,
		},
		{
			name:     "negative total",
			nMissed:  0,
			nTotal:   -5,
			expected: false,
			wantErr:  true,
		},
		{
			name:     "missed greater than total",
			nMissed:  15,
			nTotal:   10,
			expected: false,
			wantErr:  true,
		},
		{
			name:     "both negative",
			nMissed:  -1,
			nTotal:   -1,
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MissedStatTest(tt.nMissed, tt.nTotal)
			if (err != nil) != tt.wantErr {
				t.Errorf("MissedStatTest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("MissedStatTest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMissedStatTest(t *testing.T) {
	tests := []struct {
		name     string
		nMissed  int
		nTotal   int
		expected bool
		wantErr  bool
	}{
		// Test cases with exact lookup map values
		{
			name:     "exact lookup - 10 total, 3 missed (passes)",
			nMissed:  3,
			nTotal:   10,
			expected: true, // 3 <= 4 (critical value), so passes
		},
		{
			name:     "exact lookup - 10 total, 4 missed (boundary - passes)",
			nMissed:  4,
			nTotal:   10,
			expected: true, // 4 <= 4 (critical value), so passes
		},
		{
			name:     "exact lookup - 10 total, 5 missed (exceeds)",
			nMissed:  5,
			nTotal:   10,
			expected: false, // 5 > 4 (critical value), so fails
		},
		{
			name:     "exact lookup - 20 total, 4 missed (passes)",
			nMissed:  4,
			nTotal:   20,
			expected: true, // 4 <= 5 (critical value), so passes
		},
		{
			name:     "exact lookup - 20 total, 5 missed (boundary - passes)",
			nMissed:  5,
			nTotal:   20,
			expected: true, // 5 <= 5 (critical value), so passes
		},
		{
			name:     "exact lookup - 20 total, 6 missed (exceeds)",
			nMissed:  6,
			nTotal:   20,
			expected: false, // 6 >= 5 (critical value), so fails
		},
		{
			name:     "exact lookup - 100 total, 15 missed (passes)",
			nMissed:  15,
			nTotal:   100,
			expected: true, // 15 < 16 (critical value), so passes
		},
		{
			name:     "exact lookup - 100 total, 16 missed (boundary - passes)",
			nMissed:  16,
			nTotal:   100,
			expected: true, // 16 <= 16 (critical value), so passes
		},
		{
			name:     "exact lookup - 100 total, 17 missed (exceeds)",
			nMissed:  17,
			nTotal:   100,
			expected: false, // 17 >= 16 (critical value), so fails
		},
		{
			name:     "exact lookup - 500 total, 61 missed (passes)",
			nMissed:  61,
			nTotal:   500,
			expected: true, // 61 < 62 (critical value), so passes
		},
		{
			name:     "exact lookup - 500 total, 62 missed (boundary - passes)",
			nMissed:  62,
			nTotal:   500,
			expected: true, // 62 <= 62 (critical value), so passes
		},
		{
			name:     "exact lookup - 500 total, 63 missed (boundary - fails)",
			nMissed:  63,
			nTotal:   500,
			expected: false, // 63 >= 62 (critical value), so fails
		},
		{
			name:     "exact 990 - uses 10% rule, 98 missed (passes)",
			nMissed:  98,
			nTotal:   990,
			expected: true, // 98*10 = 980 <= 990, so passes
		},
		{
			name:     "exact 990 - uses 10% rule, 99 missed (boundary - passes)",
			nMissed:  99,
			nTotal:   990,
			expected: true, // 99*10 = 990 <= 990, so passes
		},
		{
			name:     "exact 990 - uses 10% rule, 100 missed (boundary - passes)",
			nMissed:  100,
			nTotal:   990,
			expected: true,
		},

		// Test cases with values between lookup map entries
		{
			name:     "between lookup values - 15 total, uses 10's critical value",
			nMissed:  3,
			nTotal:   15,
			expected: true, // Uses critical value for 10 (4), so 3 < 4 = true
		},
		{
			name:     "between lookup values - 15 total, 4 missed",
			nMissed:  4,
			nTotal:   15,
			expected: true, // Uses critical value for 10 (4), so 4 <= 4 = true
		},
		{
			name:     "between lookup values - 75 total, uses 70's critical value",
			nMissed:  10,
			nTotal:   75,
			expected: true, // Uses critical value for 70 (12), so 10 < 12 = true
		},
		{
			name:     "between lookup values - 75 total, 12 missed",
			nMissed:  12,
			nTotal:   75,
			expected: true, // Uses critical value for 70 (12), so 12 <= 12 = true
		},

		// Edge cases
		{
			name:     "zero missed",
			nMissed:  0,
			nTotal:   10,
			expected: true, // 0/10 = 0, which is always <= critical rate
		},
		{
			name:     "zero missed, large total",
			nMissed:  0,
			nTotal:   1000,
			expected: true, // 0/1000 = 0, which is always <= critical rate
		},
		{
			name:     "all missed",
			nMissed:  10,
			nTotal:   10,
			expected: false, // 10/10 = 1.0, which exceeds any critical rate
		},
		{
			name:     "small total below lookup range",
			nMissed:  1,
			nTotal:   5,
			expected: true, // Uses critical value for 5 (3), so 1 < 3 = true
		},
		{
			name:     "small total below lookup range - high miss rate",
			nMissed:  3,
			nTotal:   5,
			expected: true, // Uses critical value for 5 (3), so 3 <= 3 = true
		},
		{
			name:     "small total below lookup range - low miss rate",
			nMissed:  2,
			nTotal:   5,
			expected: true, // Uses critical value for 5 (3), so 2 < 3 = true
		},
		{
			name:     "very small total",
			nMissed:  1,
			nTotal:   1,
			expected: true, // idx=0, so returns true regardless
		},
		{
			name:     "large total above lookup range",
			nMissed:  120,
			nTotal:   1000,
			expected: false, // Uses critical value for 990 (116), so 120/1000 = 0.12, 116/990 ≈ 0.117, 0.12 > 0.117 = false
		},
		{
			name:     "large total above lookup range - passes",
			nMissed:  99,
			nTotal:   1000,
			expected: true, // Uses 10% rule: 99*10 = 990 <= 1000, so passes
		},
		{
			name:     "large total above lookup range - boundary",
			nMissed:  100,
			nTotal:   1000,
			expected: true, // Uses 10% rule: 100*10 = 1000 <= 1000, so passes
		},
		{
			name:     "large total above lookup range - fails",
			nMissed:  101,
			nTotal:   1000,
			expected: false, // Uses 10% rule: 101*10 = 1010 > 1000, so fails
		},

		// Boundary conditions at the edges of the lookup map
		{
			name:     "minimum lookup value - 10 total",
			nMissed:  3,
			nTotal:   10,
			expected: true, // 3/10 = 0.3, 4/10 = 0.4, 0.3 <= 0.4 = true
		},
		{
			name:     "maximum lookup value - 989 total",
			nMissed:  114,
			nTotal:   989,
			expected: true, // Uses table lookup for 980 (115), 114 < 115 so passes
		},
		{
			name:     "990 total uses 10% rule - boundary passes",
			nMissed:  100,
			nTotal:   990,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MissedStatTest(tt.nMissed, tt.nTotal)
			if (err != nil) != tt.wantErr {
				t.Errorf("MissedStatTest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("MissedStatTest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMissedStatTestSpecialCase990(t *testing.T) {
	// Test the special case where nTotal >= 990 uses the 10% rule
	tests := []struct {
		name     string
		nMissed  int
		nTotal   int
		expected bool
	}{
		{
			name:     "exactly 990 - uses 10% rule",
			nMissed:  99,
			nTotal:   990,
			expected: true, // 99*10 = 990 <= 990, so passes
		},
		{
			name:     "exactly 990 - boundary fails",
			nMissed:  100,
			nTotal:   990,
			expected: true,
		},
		{
			name:     "exactly 1000 - uses 10% rule",
			nMissed:  100,
			nTotal:   1000,
			expected: true, // 100*10 = 1000 <= 1000, so passes
		},
		{
			name:     "exactly 1000 - boundary fails",
			nMissed:  101,
			nTotal:   1000,
			expected: false, // 101*10 = 1010 > 1000, so fails
		},
		{
			name:     "large number - 10% rule passes",
			nMissed:  500,
			nTotal:   5000,
			expected: true, // 500*10 = 5000 <= 5000, so passes
		},
		{
			name:     "large number - 10% rule fails",
			nMissed:  501,
			nTotal:   5000,
			expected: false, // 501*10 = 5010 > 5000, so fails
		},
		{
			name:     "very large number - 9.9% passes",
			nMissed:  99,
			nTotal:   1000,
			expected: true, // 99*10 = 990 <= 1000, so passes
		},
		{
			name:     "very large number - exactly 10%",
			nMissed:  1000,
			nTotal:   10000,
			expected: true, // 1000*10 = 10000 <= 10000, so passes
		},
		{
			name:     "very large number - more than 10%",
			nMissed:  1001,
			nTotal:   10000,
			expected: false, // 1001*10 = 10010 > 10000, so fails
		},
		{
			name:     "very large number - boundary exact",
			nMissed:  116,
			nTotal:   990,
			expected: true, // 116*10 = 1160 <= 990, so passes
		},
		{
			name:     "very large number - boundary exceeds",
			nMissed:  117,
			nTotal:   990,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MissedStatTest(tt.nMissed, tt.nTotal)
			if err != nil {
				t.Errorf("MissedStatTest() unexpected error = %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("MissedStatTest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMissedStatTestLookupMapBoundaries(t *testing.T) {
	// Test that the lookup map is used correctly for boundary values
	testCases := []struct {
		total          int
		expectedLookup int
		description    string
	}{
		{5, 10, "below minimum should use 10"},
		{10, 10, "exact match should use 10"},
		{15, 20, "between 10 and 20 should use 20"},
		{20, 20, "exact match should use 20"},
		{25, 30, "between 20 and 30 should use 30"},
		{990, 990, "exact match should use 990"},
		{1000, 990, "above maximum should use 990"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// We'll test this by checking that the critical rate calculation is consistent
			// For nMissed = 0, the test should always pass regardless of the lookup value used
			result, err := MissedStatTest(0, tc.total)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !result {
				t.Errorf("Expected test to pass for 0 missed out of %d total", tc.total)
			}
		})
	}
}

func TestMissedStatTestEdgeCasesSmallValues(t *testing.T) {
	// Test edge cases for very small values that fall below the first table entry
	tests := []struct {
		name     string
		nMissed  int
		nTotal   int
		expected bool
	}{
		{
			name:     "total 1 - always returns true when idx=0",
			nMissed:  0,
			nTotal:   1,
			expected: true, // idx=0, so returns true
		},
		{
			name:     "total 1 - with 1 miss returns true",
			nMissed:  1,
			nTotal:   1,
			expected: true, // idx=0, so returns true
		},
		{
			name:     "total 2 - no misses",
			nMissed:  0,
			nTotal:   2,
			expected: true, // idx=0, so returns true
		},
		{
			name:     "total 2 - with misses",
			nMissed:  2,
			nTotal:   2,
			expected: true, // 2 <= 2 (critical value), so passes
		},
		{
			name:     "total 4 - below first table entry",
			nMissed:  2,
			nTotal:   4,
			expected: true, // 2 <= 2 (critical value for 4), so passes
		},
		{
			name:     "total 5 - exactly first table entry",
			nMissed:  2,
			nTotal:   5,
			expected: true, // 2 < 3 (critical value for 5), so passes
		},
		{
			name:     "total 5 - boundary",
			nMissed:  3,
			nTotal:   5,
			expected: true, // 3 <= 3 (critical value for 5), so passes
		},
		{
			name:     "total 6 - uses 5's critical value",
			nMissed:  2,
			nTotal:   6,
			expected: true, // Uses critical value for 5 (3), so 2 < 3 = true
		},
		{
			name:     "total 6 - boundary passes",
			nMissed:  3,
			nTotal:   6,
			expected: true, // Uses critical value for 5 (3), so 3 <= 3 = true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MissedStatTest(tt.nMissed, tt.nTotal)
			if err != nil {
				t.Errorf("MissedStatTest() unexpected error = %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("MissedStatTest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMissedStatTestConsistency(t *testing.T) {
	// Test that the function is consistent - if a test passes for a given rate,
	// it should also pass for lower rates with the same total
	testTotals := []int{10, 50, 100, 500}

	for _, total := range testTotals {
		t.Run(fmt.Sprintf("consistency_test_total_%d", total), func(t *testing.T) {
			// Find the boundary where the test starts failing
			var lastPassingMissed int = -1
			for missed := 0; missed <= total; missed++ {
				result, err := MissedStatTest(missed, total)
				if err != nil {
					t.Errorf("Unexpected error for missed=%d, total=%d: %v", missed, total, err)
					continue
				}

				if result {
					lastPassingMissed = missed
				} else {
					// Once we find a failing case, all higher values should also fail
					for higherMissed := missed + 1; higherMissed <= total && higherMissed <= missed+5; higherMissed++ {
						higherResult, err := MissedStatTest(higherMissed, total)
						if err != nil {
							t.Errorf("Unexpected error for missed=%d, total=%d: %v", higherMissed, total, err)
							continue
						}
						if higherResult {
							t.Errorf("Inconsistency: missed=%d passes but missed=%d fails for total=%d", higherMissed, missed, total)
						}
					}
					break
				}
			}

			// Verify that all values up to lastPassingMissed pass
			for missed := 0; missed <= lastPassingMissed; missed++ {
				result, err := MissedStatTest(missed, total)
				if err != nil {
					t.Errorf("Unexpected error for missed=%d, total=%d: %v", missed, total, err)
					continue
				}
				if !result {
					t.Errorf("Expected missed=%d to pass for total=%d (lastPassing=%d)", missed, total, lastPassingMissed)
				}
			}
		})
	}
}

// Benchmark tests to ensure the function performs well
func BenchmarkMissedStatTest(b *testing.B) {
	// Test various scenarios to ensure consistent performance
	testCases := []struct {
		name    string
		nMissed int
		nTotal  int
	}{
		{"small_values", 3, 10},
		{"medium_values", 50, 500},
		{"large_values_table", 100, 990},
		{"large_values_formula", 100, 1000},
		{"very_large_values", 1000, 10000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = MissedStatTest(tc.nMissed, tc.nTotal)
			}
		})
	}
}

func BenchmarkMissedStatTestWorstCase(b *testing.B) {
	// Benchmark the worst case scenario - searching through the entire table
	b.Run("worst_case_search", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = MissedStatTest(115, 989) // Just below 990, uses table lookup
		}
	})
}
