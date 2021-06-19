package pin

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGuard(t *testing.T) {
	for name, test := range map[string]struct {
		input    string // empty for a bad reader
		pinCalls map[string]error
		wantErr  bool
	}{
		"pin ok": {
			input:    "1234\n",
			pinCalls: map[string]error{"1234": nil},
		},
		"errors non-fatal": {
			input:    "5678\n",
			pinCalls: map[string]error{"5678": errors.New("db problem")},
		},
		"empty lines not sent": {
			input: "\n",
		},
		"bad reader": {
			input:   "", // makes a bad reader below
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			reader := bytes.NewReader([]byte(test.input))

			p := &mockPIN{}
			p.Test(t)
			defer p.AssertExpectations(t)
			for pin, err := range test.pinCalls {
				p.On("CheckPIN", mock.Anything, int32(7), "B", pin).Return("door things", err).Once()
			}

			g := New(reader, p, 7, "B")

			err := g.guard()
			require.Equal(t, test.wantErr, err != nil, "wantErr=%t, err=%v", test.wantErr, err)
		})
	}
}

type mockPIN struct {
	mock.Mock
}

func (m *mockPIN) CheckPIN(ctx context.Context, door int32, side, pin string) (string, error) {
	args := m.Called(ctx, door, side, pin)
	return args.String(0), args.Error(1)
}
