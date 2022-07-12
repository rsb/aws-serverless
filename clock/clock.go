package clock

import "time"

// Clock is used to control time mainly for testing
type Clock struct {
	Instant time.Time
}

// Now represents the current time
func (c *Clock) Now() time.Time {
	if c == nil {
		return time.Now()
	}

	return c.Instant
}
