package strike

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/somakeit/door-controller3/admitter"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"periph.io/x/conn/v3/gpio"
)

var mockLogger = &testLogger{}

func init() {
	Logger = mockLogger
}

func TestStrikeIneffs(t *testing.T) {
	mockStrike := &testPin{}
	mockStrike.Test(t)
	defer mockStrike.AssertExpectations(t)

	var s admitter.Admitter = New(mockStrike)
	ctx := context.Background()

	s.Interrogating(ctx, "authing...")
	require.NoError(t, s.Deny(ctx, "No", admitter.AccessDenied))
}

func TestStrikeAllow(t *testing.T) {
	for name, test := range map[string]struct {
		calls int

		openErr, closeErr error

		wantOpenCalls, wantCloseCalls int
		wantErr, wantFatal            bool
	}{
		"allowed once": {
			calls:          1,
			wantOpenCalls:  1,
			wantCloseCalls: 1,
		},

		"allowed concurrently": {
			calls:          2,
			wantOpenCalls:  2,
			wantCloseCalls: 2,
		},

		"calls fatal if door fails to open": {
			calls:          1,
			openErr:        errors.New("io error"),
			wantOpenCalls:  1,
			wantCloseCalls: 1, // because testLogger does not panic we expect a call to close
			wantFatal:      true,
		},

		"calls fatal if door fails to close": {
			calls:          1,
			closeErr:       errors.New("io error"),
			wantOpenCalls:  1,
			wantCloseCalls: 1,
			wantFatal:      true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockStrike := &testPin{}
			mockStrike.Test(t)
			defer mockStrike.AssertExpectations(t)
			if test.wantCloseCalls > 0 {
				// Assert that the last call was to lock the door
				defer func() {
					require.Equal(t, gpio.Low, mockStrike.Calls[len(mockStrike.Calls)-1].Arguments.Get(0))
				}()
			}
			counterMux := sync.Mutex{}
			closeCalls := 0
			openCalls := 0
			for i := 0; i < test.wantOpenCalls; i++ {
				mockStrike.On("Out", gpio.High).Return(test.openErr).Run(func(mock.Arguments) {
					counterMux.Lock()
					defer counterMux.Unlock()
					openCalls++
				}).Once()
			}
			for i := 0; i < test.wantCloseCalls; i++ {
				mockStrike.On("Out", gpio.Low).Return(test.closeErr).Run(func(mock.Arguments) {
					counterMux.Lock()
					defer counterMux.Unlock()
					closeCalls++
				}).Once()
			}

			mockLogger.Test(t)
			mockLogger.AssertExpectations(t)
			mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
			if test.wantFatal {
				mockLogger.On("Fatal", mock.Anything, mock.Anything).Return()
			}

			s := &Strike{
				OpenFor: 100 * time.Millisecond,
				pin:     mockStrike,
				Logic:   ActiveHigh,
			}

			start := time.Now()
			for i := 0; i < test.calls; i++ {
				err := s.Allow(context.Background(), "Welcome back Bracken")
				require.Equal(t, test.wantErr, err != nil, "wantErr=%t, err=%v", test.wantErr, err)
			}

			for {
				if time.Since(start) > time.Second {
					t.Errorf("not all async calls were made within timeout: %v", mockStrike.Calls)
					break
				}
				counterMux.Lock()
				if closeCalls == test.wantCloseCalls && openCalls == test.wantOpenCalls {
					counterMux.Unlock()
					break
				}
				counterMux.Unlock()
				runtime.Gosched()
			}
			total := time.Since(start)
			require.Less(t, total, 150*time.Millisecond, "Unlocks were not handled concurrently, total=%s", total)
		})
	}
}

func TestLogDiscarder(t *testing.T) {
	require.Panics(t, func() {
		logDiscarder{}.Fatal(context.Background())
	})
}

type testPin struct {
	mock.Mock
}

func (p *testPin) Out(l gpio.Level) error {
	return p.Called(l).Error(0)
}

type testLogger struct {
	mock.Mock
}

func (l *testLogger) Fatal(ctx context.Context, args ...interface{}) {
	l.Called(ctx, args)
}

func (l *testLogger) Debug(ctx context.Context, args ...interface{}) {
	l.Called(ctx, args)
}
