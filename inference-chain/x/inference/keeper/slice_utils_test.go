package keeper

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpsertStringIntoSortedSlice_EmptyInsert(t *testing.T) {
	var list []string
	updated, found := UpsertStringIntoSortedSlice(list, "b")
	require.False(t, found)
	require.Equal(t, []string{"b"}, updated)
}

func TestUpsertStringIntoSortedSlice_InsertPositions(t *testing.T) {
	list := []string{"b", "d", "f"}
	// insert at beginning
	u, found := UpsertStringIntoSortedSlice(list, "a")
	require.False(t, found)
	require.Equal(t, []string{"a", "b", "d", "f"}, u)
	// insert in middle
	u, found = UpsertStringIntoSortedSlice(u, "c")
	require.False(t, found)
	require.Equal(t, []string{"a", "b", "c", "d", "f"}, u)
	// insert at end
	u, found = UpsertStringIntoSortedSlice(u, "z")
	require.False(t, found)
	require.Equal(t, []string{"a", "b", "c", "d", "f", "z"}, u)
}

func TestUpsertStringIntoSortedSlice_Duplicate(t *testing.T) {
	list := []string{"a", "c", "e"}
	updated, found := UpsertStringIntoSortedSlice(list, "c")
	require.True(t, found)
	// unchanged content and length
	require.Equal(t, list, updated)
}

func TestUpsertStringIntoSortedSlice_MultipleRandomOrder(t *testing.T) {
	var list []string
	inputs := []string{"delta", "alpha", "charlie", "bravo", "alpha", "echo", "bravo"}
	set := map[string]struct{}{}
	for _, v := range inputs {
		var found bool
		list, found = UpsertStringIntoSortedSlice(list, v)
		if _, ok := set[v]; ok {
			require.True(t, found)
		} else {
			require.False(t, found)
			set[v] = struct{}{}
		}
		// Ensure list is always sorted after each insertion
		require.True(t, sort.StringsAreSorted(list))
	}
	// Final list should be the sorted unique values
	expected := make([]string, 0, len(set))
	for k := range set {
		expected = append(expected, k)
	}
	sort.Strings(expected)
	require.Equal(t, expected, list)
}

func TestUpsertIntoSortedSlice_Ints_CustomOrder(t *testing.T) {
	var list []int
	less := func(a, b int) bool { return a < b }
	equal := func(a, b int) bool { return a == b }
	// Insert many values including duplicates in random order
	vals := []int{5, 3, 9, 1, 3, 7, 2, 8, 5, 0}
	for _, v := range vals {
		list, _ = UpsertIntoSortedSlice[int](list, v, less, equal)
		// Ensure always sorted ascending
		for i := 1; i < len(list); i++ {
			require.LessOrEqual(t, list[i-1], list[i])
		}
	}
	// Verify contents equal to sorted unique set
	uniq := map[int]struct{}{}
	for _, v := range vals {
		uniq[v] = struct{}{}
	}
	expected := make([]int, 0, len(uniq))
	for k := range uniq {
		expected = append(expected, k)
	}
	sort.Ints(expected)
	require.Equal(t, expected, list)
}

func TestUpsertIntoSortedSlice_Ints_DescendingOrder(t *testing.T) {
	// Test with a custom descending comparator
	var list []int
	less := func(a, b int) bool { return a > b } // reversed: greater means "less"
	equal := func(a, b int) bool { return a == b }
	vals := rand.Perm(20)
	for _, v := range vals {
		list, _ = UpsertIntoSortedSlice[int](list, v, less, equal)
		// Ensure descending order
		for i := 1; i < len(list); i++ {
			require.GreaterOrEqual(t, list[i-1], list[i])
		}
	}
}
