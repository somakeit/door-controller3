package guard

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMux(t *testing.T) {
	g1Done := make(chan struct{})
	g1 := &mockGuard{}
	g1.Test(t)
	defer g1.AssertExpectations(t)
	g1.On("Guard").Return(nil).Run(func(mock.Arguments) {
		time.Sleep(100 * time.Millisecond)
		close(g1Done)
	}).Once()
	g2 := &mockGuard{}
	g2.Test(t)
	defer g2.AssertExpectations(t)
	g2.On("Guard").Return(errors.New("oops")).Once()

	g := Mux{g1, g2}

	require.Error(t, g.Guard())

	<-g1Done
}

type mockGuard struct {
	mock.Mock
}

func (m *mockGuard) Guard() error {
	return m.Called().Error(0)
}
