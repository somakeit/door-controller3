package nfc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/somakeit/door-controller3/admitters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	strUID = "0001f680"
	rawUID = []byte{0x00, 0x01, 0xf6, 0x80}
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
			wantDenyReason:       admitters.AccessDenied,
		},

		"tag denied without message": {
			// This is what HMS actually does
			allow:    false,
			allowMsg: "",

			wantInterrogatingMsg: "Authorizing tag...",
			wantDenyMsg:          "Access denied",
			wantDenyReason:       admitters.AccessDenied,
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
				reader:      readerDobule,
				auth:        authDouble,
				gate:        admitDouble,
				ReadTimeout: 100 * time.Millisecond,
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
			secondTag: []byte{0x00, 0x01, 0xf4, 0xa9},
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
				reader:      readerDobule,
				auth:        authDouble,
				gate:        mockAdmit,
				ReadTimeout: 100 * time.Millisecond,
				AuthTimeout: 30 * time.Second,
			}

			require.NoError(t, nfc.guard())
		})
	}
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
		got := ctx.Value(admitters.Door)
		if !assert.Equal(t, int32(7), got, "Context missing expected Door, got '%s' but wanted '%s'", got, int32(7)) {
			return false
		}
		got = ctx.Value(admitters.Side)
		if !assert.Equal(t, "B", got, "Context missing expected Side, got '%s' but wanted '%s'", got, "B") {
			return false
		}
		got = ctx.Value(admitters.ID)
		if !assert.Equal(t, uid, got, "Context missing expected ID, got '%s' but wanted '%s'", got, uid) {
			return false
		}
		got = ctx.Value(admitters.Type)
		return assert.Equal(t, guardType, got, "Context missing expected Type, got '%s' but wanted '%s'", got, guardType)
	}
}
