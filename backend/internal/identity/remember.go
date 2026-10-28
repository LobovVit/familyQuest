package identity

import (
	"errors"
	"net/url"
	"time"
)

// ParseRemember accepts only supported durations, once, at the start of login.
// The choice travels in the server-side flow, never in a caller-controlled callback.
func ParseRemember(q url.Values) (int, error) {
	if len(q["remember"]) > 1 {
		return 0, errors.New("ambiguous remember duration")
	}
	switch q.Get("remember") {
	case "", "0":
		return 0, nil
	case "1", "30":
		return 30, nil // Compatibility with existing 30-day links.
	case "60":
		return 60, nil
	default:
		return 0, errors.New("invalid remember duration")
	}
}
func (s Subject) SessionTTL() time.Duration {
	if s.RememberDays == 30 || s.RememberDays == 60 {
		return time.Duration(s.RememberDays) * 24 * time.Hour
	}
	return 12 * time.Hour
}
func (s Subject) CookieMaxAge() int {
	if s.RememberDays == 30 || s.RememberDays == 60 {
		return int(s.SessionTTL().Seconds())
	}
	return 0
}
