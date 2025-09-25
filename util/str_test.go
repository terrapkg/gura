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

import "testing"

func TestSplitOnce(t *testing.T) {
	left, right := SplitOnce("foo:bar:baz", ':')
	if left != "foo" || right != "bar:baz" {
		t.Errorf("SplitOnce failed: got (%q, %q), want (\"foo\", \"bar:baz\")", left, right)
	}
	left, right = SplitOnce("foobar", ':')
	if left != "foobar" || right != "" {
		t.Errorf("SplitOnce failed: got (%q, %q), want (\"foobar\", \"\")", left, right)
	}
}

func TestSanitizeWhite(t *testing.T) {
	got := SanitizeWhite("hello\nworld\rand\tspace")
	want := "hello world and\tspace"
	if got != want {
		t.Errorf("SanitizeWhite failed: got %q, want %q", got, want)
	}
}
