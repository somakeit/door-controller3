package led

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/somakeit/door-controller3/admitter"
	"github.com/stretchr/testify/mock"
	"periph.io/x/conn/v3/gpio"
)

func TestLED(t *testing.T) {
	// This test uses one instance of LED because the background thread cannot
	// be stopped. It is started with a long off heartbeat duration and will
	// remain idle until each subtest stimulates it. One goroutine will be left
	// running until all tests in this module complete.
	defer func(r map[int]blink) { rate = r }(rate)
	rate = map[int]blink{
		heartbeat:     {0, time.Hour},
		interrogating: {time.Millisecond, time.Millisecond},
		allowed:       {time.Millisecond, 0},
		denied:        {0, time.Millisecond},
	}
	defer func(time time.Duration) { allowedTime = time }(allowedTime)
	allowedTime = time.Millisecond
	defer func(time time.Duration) { deniedTime = time }(deniedTime)
	deniedTime = time.Millisecond

	LEDDouble := &LEDstub{}
	l := New(LEDDouble)
	for {
		if LEDDouble.called {
			break
		}
		runtime.Gosched()
	}

	for name, test := range map[string]struct {
		calls func(l *LED, m *LEDmock)
	}{
		"blinks once when allowed": { // as long as allowedTime is smaller than the total allowed blink period.
			calls: func(l *LED, m *LEDmock) {
				m.On("Out", gpio.High).Return(nil).Once()
				m.On("Out", gpio.Low).Return(nil).Once()

				_ = l.Allow(context.Background(), "Welcome back Bracken")

				time.Sleep(50 * time.Millisecond)
			},
		},

		"blinks once when allowed several times quickly": {
			calls: func(l *LED, m *LEDmock) {
				// Re-turns the light on each time
				m.On("Out", gpio.High).Return(nil).Times(4)
				// Turns it off once when returning to heartbeat
				m.On("Out", gpio.Low).Return(nil).Once()

				_ = l.Allow(context.Background(), "Welcome back Bracken")
				_ = l.Allow(context.Background(), "Welcome back Bracken")
				_ = l.Allow(context.Background(), "Welcome back Bracken")
				_ = l.Allow(context.Background(), "Welcome back Bracken")

				time.Sleep(50 * time.Millisecond)
			},
		},

		"does not blink when denied": {
			calls: func(l *LED, m *LEDmock) {
				_ = l.Deny(context.Background(), "Go away", errors.New("nah mon"))
				// one extra call to off on return to heartbeat
				m.On("Out", gpio.Low).Return(nil).Twice()

				time.Sleep(50 * time.Millisecond)
			},
		},

		"blinks until context is cancelled on interrogating": {
			calls: func(l *LED, m *LEDmock) {
				ctx, cancel := context.WithCancel(context.Background())

				m.On("Out", gpio.High).Return(nil).Times(3)
				m.On("Out", gpio.Low).Return(nil).Times(3)
				m.On("Out", gpio.High).Run(func(mock.Arguments) { cancel() }).Return(nil).Once()
				// 		// one extra call to off on return to heartbeat
				m.On("Out", gpio.Low).Return(nil).Twice()

				l.Interrogating(ctx, "checking...")

				time.Sleep(50 * time.Millisecond)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockLED := &LEDmock{}
			mockLED.Test(t)
			defer mockLED.AssertExpectations(t)
			defer func(p Pin) { l.pin = p }(l.pin)
			l.pin = mockLED

			test.calls(l, mockLED)

			l.lastAllow = time.Now().Add(-time.Hour)
			l.lastDeny = time.Now().Add(-time.Hour)
		})
	}
}

func TestLEDIsAdmitter(t *testing.T) {
	var _ admitter.Admitter = &LED{}
}

type LEDmock struct {
	mock.Mock
}

func (l *LEDmock) Out(level gpio.Level) error {
	return l.Called(level).Error(0)
}

type LEDstub struct {
	called bool
}

func (l *LEDstub) Out(level gpio.Level) error {
	l.called = true
	return nil
}
