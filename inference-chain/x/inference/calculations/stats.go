package calculations

import (
	"errors"
	"sort"
)

// Threshold represents a critical value for a given sample size.
type Threshold struct {
	Total          int
	CriticalMisses int
}

// The values here correspond to p0=0.10 and alpha=0.05.
var criticalValueTable = []Threshold{
	{2, 2},
	{3, 2},
	{4, 2},
	{5, 3},
	{10, 4},
	{20, 5},
	{30, 7},
	{40, 8},
	{50, 10},
	{60, 11},
	{70, 12},
	{80, 14},
	{90, 15},
	{100, 16},
	{110, 17},
	{120, 19},
	{130, 20},
	{140, 21},
	{150, 22},
	{160, 23},
	{170, 25},
	{180, 26},
	{190, 27},
	{200, 28},
	{210, 29},
	{220, 31},
	{230, 32},
	{240, 33},
	{250, 34},
	{260, 35},
	{270, 36},
	{280, 37},
	{290, 39},
	{300, 40},
	{310, 41},
	{320, 42},
	{330, 43},
	{340, 44},
	{350, 45},
	{360, 47},
	{370, 48},
	{380, 49},
	{390, 50},
	{400, 51},
	{410, 52},
	{420, 53},
	{430, 54},
	{440, 56},
	{450, 57},
	{460, 58},
	{470, 59},
	{480, 60},
	{490, 61},
	{500, 62},
	{510, 63},
	{520, 64},
	{530, 66},
	{540, 67},
	{550, 68},
	{560, 69},
	{570, 70},
	{580, 71},
	{590, 72},
	{600, 73},
	{610, 74},
	{620, 76},
	{630, 77},
	{640, 78},
	{650, 79},
	{660, 80},
	{670, 81},
	{680, 82},
	{690, 83},
	{700, 84},
	{710, 85},
	{720, 86},
	{730, 88},
	{740, 89},
	{750, 90},
	{760, 91},
	{770, 92},
	{780, 93},
	{790, 94},
	{800, 95},
	{810, 96},
	{820, 97},
	{830, 98},
	{840, 100},
	{850, 101},
	{860, 102},
	{870, 103},
	{880, 104},
	{890, 105},
	{900, 106},
	{910, 107},
	{920, 108},
	{930, 109},
	{940, 110},
	{950, 111},
	{960, 113},
	{970, 114},
	{980, 115},
	{990, 116},
}

// MissedStatTest performs a deterministic, on-chain safe check using a pre-computed lookup table.
// It returns 'true' if the number of misses is acceptable (test passes).
func MissedStatTest(nMissed, nTotal int) (bool, error) {
	if nTotal == 0 {
		return true, nil
	}
	if nMissed < 0 || nTotal < 0 || nMissed > nTotal {
		return false, errors.New("invalid input: requires 0 <= nMissed <= nTotal and nTotal > 0")
	}

	if nTotal > 990 {
		return nMissed*10 <= nTotal, nil
	}

	// Find the critical number of misses for the given nTotal using a binary search.
	idx := sort.Search(len(criticalValueTable), func(i int) bool {
		return criticalValueTable[i].Total > nTotal
	})

	// idx is the index of the first element with Total > nTotal.
	// We need the "floor" value, which is the element at idx - 1.
	if idx == 0 {
		// nTotal is smaller than the first entry in the table
		return true, nil
	}

	// The relevant threshold is the one for the largest total that is still <= nTotal.
	criticalValue := criticalValueTable[idx-1].CriticalMisses

	// The test passes if the observed number of misses is LESS THAN OR EQUAL TO the critical value.
	// If nMissed is greater than the critical value, the test fails.
	return nMissed <= criticalValue, nil
}
