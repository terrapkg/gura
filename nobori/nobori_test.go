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
