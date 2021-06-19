package staticauth

import (
	"context"
	"time"
)

// Static is a very basic Authorizer for testing
type Static struct {
	Delay time.Duration
	Allow []string
}

func (s *Static) Allowed(ctx context.Context, door int32, side, id string) (allowed bool, message string, err error) {
	select {
	case <-time.After(s.Delay):
	case <-ctx.Done():
		return false, "", ctx.Err()
	}
	for _, a := range s.Allow {
		if a == id {
			return true, "Welcome, user.", nil
		}
	}
	return false, "Be gone, stranger.", nil
}

func (s *Static) CheckPIN(ctx context.Context, door int32, side, pin string) (string, error) {
	select {
	case <-time.After(s.Delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	for _, a := range s.Allow {
		if a == pin {
			return "Pin was good", nil
		}
	}
	return "Pin was bad", nil
}
