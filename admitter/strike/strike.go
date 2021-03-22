// strike is an Admitter for a strike (the part in the door frame that can
// allow a door to be opened)
package strike

import (
	"context"
	"fmt"
	"sync"
	"time"

	"periph.io/x/periph/conn/gpio"
)

const (
	defaultOpenTimeS = 5
)

// Pin is a GPIO pin attached to the strike
type Pin interface {
	Out(gpio.Level) error
}

// LogicLevel is used to indicate the intent of the Pin, true is active
type LogicLevel map[bool]gpio.Level

var (
	ActiveHigh = LogicLevel{true: gpio.High, false: gpio.Low}
	ActiveLow  = LogicLevel{true: gpio.Low, false: gpio.High}
)

type Strike struct {
	// OpenFor is the duration to unlock the door for, default is 5 seconds.
	OpenFor time.Duration
	// Logic is either ActiveHigh or ActiveLow, active being unlocked. The
	// default is ActiveHigh.
	Logic LogicLevel

	mux sync.Mutex
	pin Pin
}

func New(strike Pin) *Strike {
	return &Strike{
		OpenFor: defaultOpenTimeS,
		pin:     strike,
		Logic:   ActiveHigh,
	}
}

// Interrogating has no effect on a strike
func (s *Strike) Interrogating(context.Context, string) {}

// Deny has no effect on a strike
func (s *Strike) Deny(context.Context, string, error) error { return nil }

// Allow will open the strike for Strike.OpenTime
func (s *Strike) Allow(ctx context.Context, msg string) error {
	timer := time.After(s.OpenFor)
	s.mux.Lock()
	defer s.mux.Unlock()

	if err := s.pin.Out(s.Logic[true]); err != nil {
		// Try to lock the door even though I/O apparently failed
		errS := s.pin.Out(s.Logic[false])
		return fmt.Errorf("failed to unlock strike: %w (safety lock: %s)", err, errS)
	}

	<-timer

	if err := s.pin.Out(s.Logic[false]); err != nil {
		return fmt.Errorf("failed to lock strike: %w", err)
	}

	return nil
}
