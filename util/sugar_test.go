package util

import (
	"reflect"
	"strconv"
	"testing"
)

func TestSliceMap(t *testing.T) {
	arr := []int{1, 2, 3}
	want := []string{"1", "2", "3"}
	got := SliceMap(arr, func(i int) string { return strconv.Itoa(i) })
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SliceMap failed: got %v, want %v", got, want)
	}
}

func TestSliceEach(t *testing.T) {
	arr := []int{1, 2, 3}
	sum := 0
	SliceEach(arr, func(i int) { sum += i })
	if sum != 6 {
		t.Errorf("SliceEach failed: got sum=%v, want 6", sum)
	}
}
