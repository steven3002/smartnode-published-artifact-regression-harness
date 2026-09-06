package health

import (
	"testing"
	"time"
)

func TestRestartSeries_HasStabilised(t *testing.T) {
	window := 10 * time.Second
	now := time.Now()

	tests := []struct {
		name     string
		series   RestartSeries
		expected bool
	}{
		{
			name:     "empty series",
			series:   RestartSeries{},
			expected: false,
		},
		{
			name: "currently not running",
			series: RestartSeries{
				{IsRunning: false, RestartCount: 0, ObservedAt: now.Add(-15 * time.Second)},
			},
			expected: false,
		},
		{
			name: "running but not for long enough",
			series: RestartSeries{
				{IsRunning: true, RestartCount: 0, ObservedAt: now.Add(-5 * time.Second)},
			},
			expected: false,
		},
		{
			name: "running and stable for window",
			series: RestartSeries{
				{IsRunning: true, RestartCount: 0, ObservedAt: now.Add(-15 * time.Second)},
				{IsRunning: true, RestartCount: 0, ObservedAt: now},
			},
			expected: true,
		},
		{
			name: "healthy 2-4 restart cold start (C22)",
			series: RestartSeries{
				// Restarts incrementing
				{IsRunning: true, RestartCount: 1, ObservedAt: now.Add(-30 * time.Second)},
				{IsRunning: true, RestartCount: 2, ObservedAt: now.Add(-25 * time.Second)},
				{IsRunning: true, RestartCount: 3, ObservedAt: now.Add(-20 * time.Second)},
				// Now it stops restarting and stabilizes
				{IsRunning: true, RestartCount: 3, ObservedAt: now.Add(-15 * time.Second)}, // Meets the window condition when compared to now
				{IsRunning: true, RestartCount: 3, ObservedAt: now.Add(-5 * time.Second)},
				{IsRunning: true, RestartCount: 3, ObservedAt: now},
			},
			expected: true,
		},
		{
			name: "genuine wedged stack - runaway restarts",
			series: RestartSeries{
				{IsRunning: true, RestartCount: 4, ObservedAt: now.Add(-20 * time.Second)},
				{IsRunning: true, RestartCount: 5, ObservedAt: now.Add(-15 * time.Second)},
				{IsRunning: true, RestartCount: 6, ObservedAt: now.Add(-10 * time.Second)},
				{IsRunning: true, RestartCount: 7, ObservedAt: now.Add(-5 * time.Second)},
				{IsRunning: true, RestartCount: 8, ObservedAt: now},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.series.HasStabilised(window); got != tt.expected {
				t.Errorf("HasStabilised() = %v, want %v", got, tt.expected)
			}
		})
	}
}
