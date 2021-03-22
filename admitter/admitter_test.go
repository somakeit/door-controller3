package admitter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMuxInterrogating(t *testing.T) {
	var m Mux
	for i := 0; i < 10; i++ {
		mockAdmitter := &testAdmitter{}
		mockAdmitter.Test(t)
		defer func(i int) {
			if !mockAdmitter.AssertExpectations(t) {
				t.Errorf("Expectations not met for admitter %d", i)
			}
		}(i)
		mockAdmitter.On("Interrogating", mock.Anything, "Authorizing tag...").Return().Once()
		m = append(m, mockAdmitter)
	}
	m.Interrogating(context.Background(), "Authorizing tag...")
}

func TestMuxDeny(t *testing.T) {
	for name, test := range map[string]struct {
		errIndex int
		wantErr  bool
	}{
		"all work": {
			errIndex: 11,
		},

		"three fails": {
			errIndex: 3,
			wantErr:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var m Mux
			for i := 0; i < 10; i++ {
				mockAdmitter := &testAdmitter{}
				mockAdmitter.Test(t)
				defer func(i int) {
					if !mockAdmitter.AssertExpectations(t) {
						t.Errorf("Expectations not met for admitter %d", i)
					}
				}(i)
				if i < test.errIndex {
					mockAdmitter.On("Deny", mock.Anything, "Welcome back Bracken", AccessDenied).Return(nil).Once()
				} else if i == test.errIndex {
					mockAdmitter.On("Deny", mock.Anything, "Welcome back Bracken", AccessDenied).Return(errors.New("fatal")).Once()
				}
				m = append(m, mockAdmitter)
			}
			err := m.Deny(context.Background(), "Welcome back Bracken", AccessDenied)
			require.Equal(t, test.wantErr, err != nil, "want error=%t, err=%v", test.wantErr, err)
		})
	}
}

func TestMuxAllow(t *testing.T) {
	for name, test := range map[string]struct {
		errIndex int
		wantErr  bool
	}{
		"all work": {
			errIndex: 11,
		},

		"seven fails": {
			errIndex: 7,
			wantErr:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var m Mux
			for i := 0; i < 10; i++ {
				mockAdmitter := &testAdmitter{}
				mockAdmitter.Test(t)
				defer func(i int) {
					if !mockAdmitter.AssertExpectations(t) {
						t.Errorf("Expectations not met for admitter %d", i)
					}
				}(i)
				if i < test.errIndex {
					mockAdmitter.On("Allow", mock.Anything, "Welcome back Bracken").Return(nil).Once()
				} else if i == test.errIndex {
					mockAdmitter.On("Allow", mock.Anything, "Welcome back Bracken").Return(errors.New("fatal")).Once()
				}
				m = append(m, mockAdmitter)
			}
			err := m.Allow(context.Background(), "Welcome back Bracken")
			require.Equal(t, test.wantErr, err != nil, "want error=%t, err=%v", test.wantErr, err)
		})
	}
}

type testAdmitter struct {
	mock.Mock
}

func (a *testAdmitter) Interrogating(ctx context.Context, msg string) {
	a.Called(ctx, msg)
}

func (a *testAdmitter) Deny(ctx context.Context, msg string, reason error) error {
	return a.Called(ctx, msg, reason).Error(0)
}

func (a *testAdmitter) Allow(ctx context.Context, msg string) error {
	return a.Called(ctx, msg).Error(0)
}
