package pin

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/somakeit/door-controller3/admitter"
)

const (
	pinTimeout = 30 * time.Second
	guardType  = "pin"
)

// Logger can be used to interface any logger to this package, by default
// it discards all logs and panics on Fatal calls.
var Logger ContextLogger = logDiscarder{}

// ContextLogger is the logger needed by pin
type ContextLogger interface {
	Info(ctx context.Context, args ...interface{})
	Error(ctx context.Context, args ...interface{})
}

type logDiscarder struct{}

func (logDiscarder) Info(context.Context, ...interface{})  {}
func (logDiscarder) Error(context.Context, ...interface{}) {}

// PINChecker will check a PIN and assign a tag if approproate and return a
// message to be displayed.
type PINChecker interface {
	CheckPIN(ctx context.Context, door int32, side, pin string) (string, error)
}

// Guard is a pin code rader for the HMS Guardian system, it takes pin codes
// from a reader terminated by "\n" and sends them to HMS.
type Guard struct {
	in   *bufio.Reader
	hms  PINChecker
	door int32
	side string
}

// New returns a Guard, in must be a pin souce, usually STDIN.
func New(in io.Reader, hms PINChecker, door int32, side string) *Guard {
	return &Guard{
		in:   bufio.NewReader(in),
		hms:  hms,
		door: door,
		side: side,
	}
}

// Guard begins waiting for pin codes, any errors returned are fatal.
func (g *Guard) Guard() error {
	for {
		if err := g.guard(); err != nil {
			return err
		}
	}
}

func (g *Guard) guard() error {
	ctx := context.Background()
	ctx = context.WithValue(ctx, admitter.Door, g.door)
	ctx = context.WithValue(ctx, admitter.Side, g.side)
	ctx = context.WithValue(ctx, admitter.Type, guardType)

	fmt.Print("Enter pin: ")
	pin, err := g.in.ReadString('\n')
	if err != nil {
		Logger.Error(ctx, "Error reading pin: ", err)
		return fmt.Errorf("failed to read pin: %w", err)
	}
	pin = strings.TrimSuffix(pin, "\n")
	if pin == "" {
		return nil
	}

	ctx = context.WithValue(ctx, admitter.ID, pin)
	ctx, cancel := context.WithTimeout(ctx, pinTimeout)
	defer cancel()
	msg, err := g.hms.CheckPIN(ctx, g.door, g.side, pin)
	if err != nil {
		Logger.Error(ctx, "PIN check failed: ", err)
		fmt.Println("PIN check failed:", err)
		return nil
	}
	Logger.Info(ctx, "PIN OK: ", msg)
	fmt.Println("PIN OK:", msg)
	return nil
}
