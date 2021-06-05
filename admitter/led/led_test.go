package led

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"periph.io/x/conn/v3/gpio"
)

func TestLED(t *testing.T) {
	type input struct {
		after time.Duration
		do    func(*LED)
		done  bool
	}
	for name, test := range map[string]struct {
		allowedTime, deniedTime time.Duration
		rates                   map[int]blink
		calls                   []gpio.Level
		inputs                  []input
	}{
		"blinks when nothing happens": {
			// twice in 1 second: 0-on, 300-off, 600-on, 900-off...
			rates: map[int]blink{
				heartbeat: {on: 300 * time.Millisecond, off: 300 * time.Millisecond},
			},
			calls: []gpio.Level{gpio.High, gpio.Low, gpio.High, gpio.Low},
		},

		"doesn't blink on if disabled": {
			// off twice in 1 second: 0-off, 600-off...
			rates: map[int]blink{
				heartbeat: {on: 0, off: 600 * time.Millisecond},
			},
			calls: []gpio.Level{gpio.Low, gpio.Low},
		},

		"blinks once when allowed": { // as long as allowedTime is smaller than the total allowed blink period.
			// on for half a second: 0-off, 50-Allow, 50-on, 300-on, 550-off, 650-off, 750-off, 850-off, 950-off...
			allowedTime: 500 * time.Millisecond,
			rates: map[int]blink{
				heartbeat: {on: 0, off: 100 * time.Millisecond},
				allowed:   {on: 250 * time.Millisecond, off: 0},
			},
			inputs: []input{
				{after: 50 * time.Millisecond, do: func(l *LED) { _ = l.Allow(context.Background(), "yea") }},
			},
			calls: []gpio.Level{gpio.Low, gpio.High, gpio.High, gpio.Low, gpio.Low, gpio.Low, gpio.Low, gpio.Low},
		},

		"blinks once when allowed several times quickly": {
			// on for half a second: 0-off, 50-Allow, 50-on, 80-Allow, 80-on, 110-Allow, 110-on, 360-on, 610-off, 710-off, 810-off, 910-off...
			allowedTime: 500 * time.Millisecond,
			rates: map[int]blink{
				heartbeat: {on: 0, off: 100 * time.Millisecond},
				allowed:   {on: 250 * time.Millisecond, off: 0},
			},
			inputs: []input{
				{after: 50 * time.Millisecond, do: func(l *LED) { _ = l.Allow(context.Background(), "yea") }},
				{after: 80 * time.Millisecond, do: func(l *LED) { _ = l.Allow(context.Background(), "yea") }},
				{after: 110 * time.Millisecond, do: func(l *LED) { _ = l.Allow(context.Background(), "yea") }},
			},
			calls: []gpio.Level{gpio.Low, gpio.High, gpio.High, gpio.High, gpio.High, gpio.Low, gpio.Low, gpio.Low, gpio.Low},
		},

		"does not blink when denied": {
			// blinks evey 100 except a 500 gap: 0-on, 10-off, 110-on, 120-off, 200-Deny, 200-off, 450-off, 700-on, 710-off, 810-on, 820-off, 920-on, 930-off...
			deniedTime: 500 * time.Millisecond,
			rates: map[int]blink{
				heartbeat: {on: 10 * time.Millisecond, off: 100 * time.Millisecond},
				denied:    {on: 0, off: 250 * time.Millisecond},
			},
			inputs: []input{
				{after: 200 * time.Millisecond, do: func(l *LED) { _ = l.Deny(context.Background(), "nah", errors.New("said no")) }},
			},
			calls: []gpio.Level{gpio.High, gpio.Low, gpio.High, gpio.Low, gpio.Low, gpio.Low, gpio.High, gpio.Low, gpio.High, gpio.Low, gpio.High, gpio.Low},
		},

		"blinks until context is cancelled on interrogating": {
			// blinks 3 times fast: 0-off, 100-off, 180-Interrogating(230), 180-on, 230-off, 280-on, 330-off, 380-on, 410-off, 510-off, 610-off, 710-off, 810-off, 910-off...
			rates: map[int]blink{
				heartbeat:     {on: 0, off: 100 * time.Millisecond},
				interrogating: {on: 50 * time.Millisecond, off: 50 * time.Millisecond},
			},
			inputs: []input{
				{after: 180 * time.Millisecond, do: func(l *LED) {
					ctx, _ := context.WithTimeout(context.Background(), 230*time.Millisecond)
					l.Interrogating(ctx, "checking...")
				}},
			},
			calls: []gpio.Level{gpio.Low, gpio.Low, gpio.High, gpio.Low, gpio.High, gpio.Low, gpio.High, gpio.Low, gpio.Low, gpio.Low, gpio.Low, gpio.Low, gpio.Low},
		},
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			pin := &pinMock{
				t:     t,
				start: start,
			}
			pin.Test(t)
			defer pin.AssertExpectations(t)
			defer func() { assertCallOrder(t, pin.Calls, test.calls) }()
			for _, level := range test.calls {
				pin.On("Out", level).Return(nil).Once()
			}

			l := &LED{
				allowedTime: test.allowedTime,
				deniedTime:  test.deniedTime,
				rate:        test.rates,
				pin:         pin,
				wake:        make(chan struct{}),
			}

			// do the inputs at the right times
			go func(inputs []input) {
				for {
					inputsLeft := false
					for i := range inputs {
						if inputs[i].done {
							continue
						}
						inputsLeft = true
						if time.Since(start) < inputs[i].after {
							continue
						}
						inputs[i].do(l)
						inputs[i].done = true
					}
					if !inputsLeft {
						break
					}
				}
			}(test.inputs)

			// runs for 1 second
			for time.Since(start) < time.Second {
				l.loop()
			}
		})
	}
}

func assertCallOrder(t *testing.T, calls []mock.Call, expected []gpio.Level) {
	assert.Len(t, calls, len(expected), "Wrong number of calls to Out(), got %d, want %d.", len(calls), len(expected))
	for i := range expected {
		assert.Equal(t, expected[i], calls[i].Arguments[0], "Call %d should be %v, got %v", i, expected[i], calls[i].Arguments[0])
	}
}

type pinMock struct {
	t     *testing.T
	start time.Time
	mock.Mock
}

func (p *pinMock) Out(level gpio.Level) error {
	p.t.Log(level, time.Since(p.start))
	return p.Called(level).Error(0)
}
