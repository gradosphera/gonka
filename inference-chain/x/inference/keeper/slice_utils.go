package keeper

import "slices"

// UpsertIntoSortedSlice inserts value into a sorted slice using the provided ordering functions.
// It uses slices.BinarySearchFunc and slices.Insert to avoid reimplementing search/insert.
// If the value already exists (as determined by equal), it returns the original slice and found=true.
// Otherwise, it inserts the value at the correct position to maintain sort order and returns the new slice with found=false.
func UpsertIntoSortedSlice[T any](list []T, value T, less func(a, b T) bool, equal func(a, b T) bool) ([]T, bool) {
	cmp := func(a, b T) int {
		if equal(a, b) {
			return 0
		}
		if less(a, b) {
			return -1
		}
		return 1
	}
	index, found := slices.BinarySearchFunc(list, value, cmp)
	if found {
		return list, true
	}
	return slices.Insert(list, index, value), false
}

// UpsertStringIntoSortedSlice is a convenience wrapper for []string using lexicographic ordering.
func UpsertStringIntoSortedSlice(list []string, value string) ([]string, bool) {
	index, found := slices.BinarySearch(list, value)
	if found {
		return list, true
	}
	return slices.Insert(list, index, value), false
}
