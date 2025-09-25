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
