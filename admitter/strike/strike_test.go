package strike

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/somakeit/door-controller3/admitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"periph.io/x/periph/conn/gpio"
)

func TestStrikeIneffs(t *testing.T) {
	mockStrike := &testPin{}
	mockStrike.Test(t)
	defer mockStrike.AssertExpectations(t)

	var s admitter.Admitter = New(mockStrike)
	ctx := context.Background()

	s.Interrogating(ctx, "authing...")
	assert.NoError(t, s.Deny(ctx, "No", admitter.AccessDenied))
}

func TestStrikeAllow(t *testing.T) {
	for name, test := range map[string]struct {
		calls             int
		openErr, closeErr error
		wantErr           bool
	}{
		"allowed once": {
			calls: 1,
		},

		"allowed concurrently": {
			calls: 2,
		},

		"failed open": {
			calls:   1,
			openErr: errors.New("io error"),
			wantErr: true,
		},

		"failed close": {
			calls:    1,
			closeErr: errors.New("io error"),
			wantErr:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockStrike := &testPin{}
			mockStrike.Test(t)
			defer mockStrike.AssertExpectations(t)
			// Assert that the last call was to lock the door
			defer func() {
				require.Equal(t, gpio.Low, mockStrike.Calls[len(mockStrike.Calls)-1].Arguments.Get(0))
			}()
			for i := 0; i < test.calls; i++ {
				mockStrike.On("Out", gpio.High).Return(test.openErr).Once()
				mockStrike.On("Out", gpio.Low).Return(test.closeErr).Once()
			}

			s := &Strike{
				OpenFor: 100 * time.Millisecond,
				pin:     mockStrike,
				Logic:   ActiveHigh,
			}

			for i := 0; i < test.calls; i++ {
				err := s.Allow(context.Background(), "Welcome back Bracken")
				require.Equal(t, test.wantErr, err != nil, "wantErr=%t, err=%v", test.wantErr, err)
			}
		})
	}
}

type testPin struct {
	mock.Mock
}

func (p *testPin) Out(l gpio.Level) error {
	return p.Called(l).Error(0)
}
