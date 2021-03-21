package nfc

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/somakeit/door-controller3/admitters"
)

const (
	defaultReadTimeoutMS = 100
	defaultAithTimeoutS  = 30
	guardType            = "nfc"
)

// UIDReader is any NFC/RFIC reader that Guard can poll for tag UIDs
type UIDReader interface {
	ReadUID(timeout time.Duration) (uid []byte, err error)
}

// Authorizer is an instance of the entity that says whether a given identifier
// is to be granted access or not. Errors from Allowed are non-fatal.
type Authorizer interface {
	Allowed(ctx context.Context, door int32, side, id string) (allowed bool, message string, err error)
}

// Guard is a a door guard for NFC tags
type Guard struct {
	door   int32
	side   string
	reader UIDReader
	auth   Authorizer
	gate   admitters.Admitter

	// ReadTimeout is the time given to read a UID from the UIDReader, the
	// default is 100 milliseconds.
	ReadTimeout time.Duration
	// AuthTimeout id the overall time given for the authorization process, if
	// this time elapses before authorization is granted, then admission will
	// be denied. The default is 30 seconds.
	AuthTimeout time.Duration
}

// New returs a new Guard, door is the id of this door, side of door is usually
// "A" or "B", reader is an instance of an NFC/RFID reader.
func New(door int32, side string, reader UIDReader, authority Authorizer, gate admitters.Admitter) (*Guard, error) {
	return &Guard{
		door:        door,
		side:        side,
		reader:      reader,
		auth:        authority,
		gate:        gate,
		ReadTimeout: defaultReadTimeoutMS * time.Millisecond,
		AuthTimeout: defaultAithTimeoutS * time.Second,
	}, nil
}

// Guard begins guarding the door. Any error returned is fatal.
func (g *Guard) Guard() error {
	for {
		if err := g.guard(); err != nil {
			return err
		}
	}
}

// guard is one iteration of the Guard loop
func (g *Guard) guard() error {
	rawUID, err := g.reader.ReadUID(g.ReadTimeout)
	if err != nil {
		// There was no tag, or we couldn't read the tag
		return nil
	}

	uid := hex.EncodeToString(rawUID)
	ctx := context.Background()
	ctx = context.WithValue(ctx, admitters.Type, guardType)
	ctx = context.WithValue(ctx, admitters.ID, uid)
	ctx, cancel := context.WithTimeout(ctx, g.AuthTimeout)

	g.gate.Interrogating(ctx, "Authorizing tag...")

	// If the admitee pulls their tag off the reader; cancel the context
	bgScan := make(chan struct{})
	defer func() { <-bgScan }()
	defer cancel()
	go func() {
		defer close(bgScan)

		for {
			if ctx.Err() != nil {
				break
			}
			rawUID, err := g.reader.ReadUID(g.ReadTimeout)
			if err != nil || uid != hex.EncodeToString(rawUID) {
				cancel()
			}
		}
	}()

	allowed, msg, err := g.auth.Allowed(ctx, g.door, g.side, uid)
	if err != nil {
		if err := g.gate.Deny(ctx, "Error", err); err != nil {
			return fmt.Errorf("failed to deny access: %w", err)
		}
		return nil
	}
	if msg == "" {
		msg = "Access denied"
	}
	if !allowed {
		if err := g.gate.Deny(ctx, msg, admitters.AccessDenied); err != nil {
			return fmt.Errorf("failed to deny access: %w", err)
		}
		return nil
	}

	if err := g.gate.Allow(ctx, msg); err != nil {
		return fmt.Errorf("failed to allow access: %w", err)
	}

	return nil
}
