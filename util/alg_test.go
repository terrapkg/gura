package util

import (
	"reflect"
	"testing"
)

func TestInsertionSort(t *testing.T) {
	arr := []int{5, 3, 4, 1, 2}
	want := []int{1, 2, 3, 4, 5}
	InsertionSort(&arr, func(a, b int) int {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	})
	if !reflect.DeepEqual(arr, want) {
		t.Errorf("InsertionSort failed: got %v, want %v", arr, want)
	}
}

func TestSortedContSearch(t *testing.T) {
	arr := []int{1, 2, 3, 4, 5}
	lastIdx := 0
	found := SortedContSearch(arr, 3, func(a, b int) int {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	}, &lastIdx)
	if !found || lastIdx != 2 {
		t.Errorf("SortedContSearch failed: found=%v, lastIdx=%v, want found=true, lastIdx=2", found, lastIdx)
	}
}

func TestMergeSortedDedup(t *testing.T) {
	slices := [][]int{{1, 2, 3}, {2, 3, 4}, {3, 4, 5}}
	want := []int{1, 2, 3, 4, 5}
	got := MergeSortedDedup(slices, func(a, b int) int {
		return a - b
	})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MergeSortedDedup failed: got %v, want %v", got, want)
	}
}
