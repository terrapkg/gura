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

package nobori

import (
	"testing"
	"time"
)

func TestCalcTimeout(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		lastChk time.Time
		wantMin time.Duration
		wantMax time.Duration
	}{
		{"Just checked", now, time.Minute, time.Minute + 1*time.Second},
		{"Checked 1 day ago", now.Add(-24 * time.Hour), 10 * time.Minute, 10*time.Minute + 1*time.Second},
		{"Checked 1 year ago", now.Add(-365 * 24 * time.Hour), 55 * time.Minute, time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcTimeout(tt.lastChk)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calcTimeout(%v) = %v, want between %v and %v", tt.lastChk, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
