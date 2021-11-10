package nfc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/somakeit/door-controller3/admitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	strUID = "0001f680"
	rawUID = []byte{0x00, 0x01, 0xf6, 0x80}
	// strAltUID = "0001f4a9"
	rawAltUID = []byte{0x00, 0x01, 0xf4, 0xa9}
)

func TestGuard(t *testing.T) {
	for name, test := range map[string]struct {
		readErr  error
		allow    bool
		allowMsg string
		allowErr error

		wantInterrogatingMsg string
		wantAllowMsg         string
		wantDenyMsg          string
		wantDenyReason       error
	}{
		"tag allowed": {
			allow:    true,
			allowMsg: "Welcome back Bracken",

			wantInterrogatingMsg: "Authorizing tag...",
			wantAllowMsg:         "Welcome back Bracken",
		},

		"tag denied": {
			allow:    false,
			allowMsg: "Unknown tag",

			wantInterrogatingMsg: "Authorizing tag...",
			wantDenyMsg:          "Unknown tag",
			wantDenyReason:       admitter.AccessDenied,
		},

		"tag denied without message": {
			// This is what HMS actually does
			allow:    false,
			allowMsg: "",

			wantInterrogatingMsg: "Authorizing tag...",
			wantDenyMsg:          "Access denied",
			wantDenyReason:       admitter.AccessDenied,
		},

		"tag allowed without message": {
			// This never happens
			allow:    true,
			allowMsg: "",

			wantInterrogatingMsg: "Authorizing tag...",
			wantAllowMsg:         "Access granted",
		},

		"error from reader": {
			readErr: errors.New("timeout"),
			// Should not do anything
		},

		"error from auth": {
			// This includes timeouts
			allowErr: errors.New("server error"),

			wantInterrogatingMsg: "Authorizing tag...",
			wantDenyMsg:          "Error",
			wantDenyReason:       errors.New("server error"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			readerDobule := &testNFC{}
			readerDobule.Test(t)
			// The reader returns an extra byte, here x.
			readerDobule.On("ReadUID", 100*time.Millisecond).Return(rawUID, test.readErr)
			authDouble := &testAuth{}
			authDouble.Test(t)
			authDouble.On("Allowed", mock.MatchedBy(contextWithUIDAndFields(t, strUID)), int32(7), "B", strUID).Return(test.allow, test.allowMsg, test.allowErr)
			mockAdmit := &testAdmit{}
			mockAdmit.Test(t)
			defer mockAdmit.AssertExpectations(t)
			if test.wantInterrogatingMsg != "" {
				mockAdmit.On("Interrogating", mock.MatchedBy(contextWithUIDAndFields(t, strUID)), test.wantInterrogatingMsg).Return().Once()
			}
			if test.wantAllowMsg != "" {
				mockAdmit.On("Allow", mock.MatchedBy(contextWithUIDAndFields(t, strUID)), test.wantAllowMsg).Return(nil).Once()
			}
			if test.wantDenyMsg != "" {
				mockAdmit.On("Deny", mock.MatchedBy(contextWithUIDAndFields(t, strUID)), test.wantDenyMsg, test.wantDenyReason).Return(nil).Once()
			}

			nfc, err := New(7, "B", readerDobule, authDouble, mockAdmit)
			require.NoError(t, err)

			require.NoError(t, nfc.guard())
		})
	}
}

func TestGuardFatal(t *testing.T) {
	for name, test := range map[string]struct {
		auth    bool
		authErr error
		gateErr error
	}{
		"fail to deny after failed auth": {
			authErr: errors.New("bad problem"),
			gateErr: errors.New("bad problem"),
		},

		"fail to deny after auth reject": {
			gateErr: errors.New("bad problem"),
		},

		"fail to allow after auth allow": {
			auth:    true,
			gateErr: errors.New("bad problem"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			readerDobule := &testNFC{}
			readerDobule.Test(t)
			readerDobule.On("ReadUID", mock.Anything).Return(rawUID, nil)
			admitDouble := &testAdmit{}
			admitDouble.Test(t)
			admitDouble.On("Interrogating", mock.Anything, mock.Anything).Return()
			admitDouble.On("Allow", mock.Anything, mock.Anything).Return(test.gateErr)
			admitDouble.On("Deny", mock.Anything, mock.Anything, mock.Anything).Return(test.gateErr)
			authDouble := &testAuth{}
			authDouble.Test(t)
			authDouble.On("Allowed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(test.auth, "", test.authErr)

			nfc := &Guard{
				reader:        readerDobule,
				auth:          authDouble,
				gate:          admitDouble,
				ReadTimeout:   100 * time.Millisecond,
				AuthTimeout:   time.Second,
				CancelTimeout: 200 * time.Millisecond,
			}

			require.Error(t, nfc.Guard())
		})
	}
}

func TestGuardUserCancel(t *testing.T) {
	for name, test := range map[string]struct {
		secondTag []byte
	}{
		"cancel because tag removed": {
			secondTag: nil,
		},

		"cancel because tag replaced": {
			secondTag: rawAltUID,
		},
	} {
		t.Run(name, func(t *testing.T) {
			readerDobule := &testNFC{}
			readerDobule.Test(t)
			readerDobule.On("ReadUID", mock.Anything).Return(rawUID, nil).Once()
			if test.secondTag != nil {
				readerDobule.On("ReadUID", mock.Anything).Return(test.secondTag, nil)
			} else {
				readerDobule.On("ReadUID", mock.Anything).After(100*time.Millisecond).Return(nil, errors.New("timeout"))
			}
			mockAdmit := &testAdmit{}
			mockAdmit.Test(t)
			mockAdmit.AssertExpectations(t)
			mockAdmit.On("Interrogating", mock.Anything, mock.Anything).Return()
			mockAdmit.On("Deny", mock.Anything, "Error", errors.New("context cancelled")).Return(nil)
			authDouble := &testAuth{}
			authDouble.Test(t)
			authDouble.On("Allowed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				select {
				case <-args.Get(0).(context.Context).Done():
				case <-time.After(3 * time.Second):
					t.Error("Expected context to be cancelled but it was not")
				}
			}).Return(false, "", errors.New("context cancelled"))

			nfc := &Guard{
				reader:        readerDobule,
				auth:          authDouble,
				gate:          mockAdmit,
				ReadTimeout:   100 * time.Millisecond,
				AuthTimeout:   30 * time.Second,
				CancelTimeout: 200 * time.Millisecond,
			}

			require.NoError(t, nfc.guard())
		})
	}
}

func TestGuardDeDupe(t *testing.T) {
	readerDobule := &testNFC{}
	readerDobule.Test(t)
	readerDobule.On("ReadUID", mock.Anything).Return(rawUID, nil)

	authDouble := &testAuth{}
	authDouble.Test(t)
	authDouble.On("Allowed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, "", nil)

	mockAdmit := &testAdmit{}
	mockAdmit.Test(t)

	nfc, err := New(1, "A", readerDobule, authDouble, mockAdmit)
	require.NoError(t, err)

	t.Run("first auth succeeds", func(t *testing.T) {
		defer mockAdmit.AssertExpectations(t)
		mockAdmit.On("Interrogating", mock.Anything, mock.Anything).Return().Maybe()
		mockAdmit.On("Allow", mock.Anything, mock.Anything).Return(nil).Once()
		require.NoError(t, nfc.guard())
	})

	t.Run("second auth by the same tag is ignored", func(t *testing.T) {
		defer mockAdmit.AssertExpectations(t)
		require.NoError(t, nfc.guard())
	})

	t.Run("same tag is allowed after a gap", func(t *testing.T) {
		defer mockAdmit.AssertExpectations(t)

		readerDobule.ExpectedCalls = nil
		readerDobule.On("ReadUID", mock.Anything).Return(nil, errors.New("no tag"))
		require.NoError(t, nfc.guard())

		readerDobule.ExpectedCalls = nil
		readerDobule.On("ReadUID", mock.Anything).Return(rawUID, nil)
		mockAdmit.On("Allow", mock.Anything, mock.Anything).Return(nil).Once()
		require.NoError(t, nfc.guard())
	})

	t.Run("different tag is allowed with no gap", func(t *testing.T) {
		defer mockAdmit.AssertExpectations(t)
		mockAdmit.On("Allow", mock.Anything, mock.Anything).Return(nil).Once()
		readerDobule.ExpectedCalls = nil
		readerDobule.On("ReadUID", mock.Anything).Return(rawAltUID, nil)
		require.NoError(t, nfc.guard())
	})
}

type testNFC struct {
	mock.Mock
}

func (n *testNFC) ReadUID(timeout time.Duration) ([]byte, error) {
	args := n.Called(timeout)
	b, _ := args.Get(0).([]byte)
	return b, args.Error(1)
}

type testAuth struct {
	mock.Mock
}

func (a *testAuth) Allowed(ctx context.Context, door int32, side, id string) (bool, string, error) {
	args := a.Called(ctx, door, side, id)
	return args.Bool(0), args.String(1), args.Error(2)
}

type testAdmit struct {
	mock.Mock
}

func (a *testAdmit) Interrogating(ctx context.Context, msg string) {
	a.Called(ctx, msg)
}

func (a *testAdmit) Deny(ctx context.Context, msg string, reason error) error {
	return a.Called(ctx, msg, reason).Error(0)
}

func (a *testAdmit) Allow(ctx context.Context, msg string) error {
	return a.Called(ctx, msg).Error(0)
}

func contextWithUIDAndFields(t *testing.T, uid string) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		got := ctx.Value(admitter.Door)
		if !assert.Equal(t, int32(7), got, "Context missing expected Door, got '%s' but wanted '%s'", got, int32(7)) {
			return false
		}
		got = ctx.Value(admitter.Side)
		if !assert.Equal(t, "B", got, "Context missing expected Side, got '%s' but wanted '%s'", got, "B") {
			return false
		}
		got = ctx.Value(admitter.ID)
		if !assert.Equal(t, uid, got, "Context missing expected ID, got '%s' but wanted '%s'", got, uid) {
			return false
		}
		got = ctx.Value(admitter.Type)
		return assert.Equal(t, guardType, got, "Context missing expected Type, got '%s' but wanted '%s'", got, guardType)
	}
}
