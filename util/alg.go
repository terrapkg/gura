/*
gura

Copyright (c) 2024-2025 Fyra Labs

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package util

// Performs insertion sort on a slice of any type using a comparison function.
// The comparison function should return:
//
//   - -1 if a < b
//   - 0 if a == b
//   - +1 if a > b
func InsertionSort[T any](arr *[]T, cmp func(T, T) int) {
	for i := range *arr {
		for j := i; j > 0 && cmp((*arr)[j-1], (*arr)[j]) == 1; j-- {
			(*arr)[j], (*arr)[j-1] = (*arr)[j-1], (*arr)[j]
		}
	}
}

// Continuous search for a target in a sorted slice using a comparison function.
// The comparison function should return:
//
//   - -1 if a < b
//   - 0 if a == b
//   - +1 if a > b
func SortedContSearch[T any, U any](arr []U, target T, cmp func(U, T) int, lastIdx *int) (found bool) {
	for i := *lastIdx; i < len(arr); i++ {
		cmp := cmp(arr[i], target)
		if cmp < 0 {
			continue
		}
		found = cmp == 0
		*lastIdx = i
		break
	}
	return
}

func MergeSortedDedup[T any](slices [][]T, cmp func(T, T) int) []T {
	idxs := make([]int, len(slices))
	var merged []T
	var last *T
	for {
		minIdx := -1
		var minPkg T
		for i, idx := range idxs {
			if idx < len(slices[i]) {
				pkg := slices[i][idx]
				if minIdx == -1 || cmp(pkg, minPkg) < 0 {
					minIdx = i
					minPkg = pkg
				}
			}
		}
		if minIdx == -1 {
			break // all slices exhausted
		}
		if last == nil || cmp(*last, minPkg) != 0 {
			merged = append(merged, minPkg)
			last = &merged[len(merged)-1]
		}
		idxs[minIdx]++
	}
	return merged
}
