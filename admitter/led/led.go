// led is a status light Admitter
package led

import (
	"context"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
)

// blink is a type to define a status light's behaviour
type blink struct {
	on, off time.Duration
}

const (
	// These are the LED states
	heartbeat = iota
	interrogating
	allowed
	denied

	defaultAllowedTime = time.Second
	defaultDeniedTime  = time.Second
)

var (
	// defaultRates maps led state to blink pattern. Every pattern must have one non-zero
	// duration
	defaultRates = map[int]blink{
		heartbeat:     {50 * time.Millisecond, 4950 * time.Millisecond},
		interrogating: {50 * time.Millisecond, 50 * time.Millisecond},
		allowed:       {time.Second, 0},
		denied:        {0, time.Second},
	}
)

// Pin is a GPIO pin attached to the LED
type Pin interface {
	Out(gpio.Level) error
}

// LED is an Admitter that impliments a status LED
type LED struct {
	allowedTime, deniedTime time.Duration
	rate                    map[int]blink

	pin Pin

	mux           sync.Mutex
	wake          chan struct{}
	interrogating bool
	lastAllow     time.Time
	lastDeny      time.Time
}

// New returns a started LED
func New(led Pin) *LED {
	l := &LED{
		allowedTime: defaultAllowedTime,
		deniedTime:  defaultDeniedTime,
		rate:        defaultRates,

		pin:  led,
		wake: make(chan struct{}),
	}
	go l.run()
	return l
}

func (l *LED) Interrogating(ctx context.Context, msg string) {
	l.mux.Lock()
	l.interrogating = true
	l.mux.Unlock()
	go func() {
		<-ctx.Done()
		l.mux.Lock()
		l.interrogating = false
		l.mux.Unlock()
		l.poke()
	}()
	l.poke()
}

func (l *LED) Deny(ctx context.Context, msg string, reason error) error {
	l.mux.Lock()
	l.lastDeny = time.Now()
	l.mux.Unlock()
	l.poke()
	return nil
}

func (l *LED) Allow(ctx context.Context, msg string) error {
	l.mux.Lock()
	l.lastAllow = time.Now()
	l.mux.Unlock()
	l.poke()
	return nil
}

// run is the background thread for LED
func (l *LED) run() {
	for {
		l.loop()
	}
}

func (l *LED) loop() {
	blink := l.rate[l.state()]

	if blink.on > 0 {
		timer := time.NewTimer(blink.on)
		_ = l.pin.Out(gpio.High)
		select {
		case <-timer.C:
		case <-l.wake:
			// stopping the timer prevents an adversary from growing a
			// gorouting horde by getting denied repeatedly.
			timer.Stop()
			return
		}
	}

	if blink.off > 0 {
		timer := time.NewTimer(blink.off)
		_ = l.pin.Out(gpio.Low)
		select {
		case <-timer.C:
		case <-l.wake:
			timer.Stop()
			return
		}
	}
}

// poke causes run to skip to the next pattern immediately
func (l *LED) poke() {
	l.wake <- struct{}{}
}

// state returns the current intended led state
func (l *LED) state() int {
	l.mux.Lock()
	defer l.mux.Unlock()
	switch {
	case time.Since(l.lastAllow) < l.allowedTime:
		return allowed
	case l.interrogating:
		return interrogating
	case time.Since(l.lastDeny) < l.deniedTime:
		return denied
	}
	return heartbeat
}
