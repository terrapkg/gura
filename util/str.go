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

func SplitOnce(s string, c byte) (left string, right string) {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			left = s[0:i]
			right = s[i+1:]
			return
		}
	}
	left = s
	return
}

// Sanitize the given string by turning whitespaces other than `\t` and ` ` to ` `.
//
// Prevent (log) injection by sanitizing input from users.
func SanitizeWhite(s string) string {
	bs := []byte(s)
	for i, b := range bs {
		if b == '\n' || b == '\r' {
			bs[i] = ' '
		}
	}
	return string(bs)
}
