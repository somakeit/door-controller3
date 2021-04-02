package contextlogger

import (
	"testing"

	"github.com/somakeit/door-controller3/admitter"
)

func TestContextLogger(t *testing.T) {
	var _ admitter.Admitter = &ContextLogger{}
}
