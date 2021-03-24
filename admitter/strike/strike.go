// strike is an Admitter for a strike (the part in the door frame that can
// allow a door to be opened)
package strike

import (
	"context"
	"fmt"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
)

const (
	defaultOpenTimeS = 5
)

// Pin is a GPIO pin attached to the strike
type Pin interface {
	Out(gpio.Level) error
}

// Logger can be used to interface any logger to this package, by default
// it discards all logs and panics on Fatal calls.
var Logger ContextLogger = logDiscarder{}

// ContextLogger is an interface which allows you to use any logger and include
// context filds.
type ContextLogger interface {
	Fatal(ctx context.Context, args ...interface{})
}

type logDiscarder struct{}

func (logDiscarder) Fatal(context.Context, ...interface{}) { panic("Fatal error in strike") }

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

// Allow will open the strike for Strike.OpenTime.
func (s *Strike) Allow(ctx context.Context, msg string) error {
	timer := time.After(s.OpenFor)

	if err := s.pin.Out(s.Logic[true]); err != nil {
		return fmt.Errorf("failed to unlock door: %w", err)
	}

	go func() {
		s.mux.Lock()
		defer s.mux.Unlock()

		<-timer

		if err := s.pin.Out(s.Logic[false]); err != nil {
			Logger.Fatal(ctx, "Failed to lock door: ", err)
		}

	}()
	return nil
}
