package domain

import (
	"testing"
	"time"
)

func TestNotificationRetryBackoff(t *testing.T) {
	for _, tc := range []struct {
		attempt int
		want    time.Duration
	}{{0, 5 * time.Second}, {1, 5 * time.Second}, {2, 10 * time.Second}, {4, 40 * time.Second}, {10, 2560 * time.Second}, {11, time.Hour}, {1000000, time.Hour}} {
		if got := NotificationRetryDelay(tc.attempt); got != tc.want {
			t.Errorf("attempt %d: %v != %v", tc.attempt, got, tc.want)
		}
	}
}
