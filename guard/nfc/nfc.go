package nfc

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/somakeit/door-controller3/admitter"
	"github.com/somakeit/door-controller3/auth"
)

const (
	defaultReadTimeoutMS = 100
	defaultAuthTimeoutS  = 30
	defaultCanelTimeoutS = 5
	guardType            = "nfc"
)

// UIDReader is any NFC/RFIC reader that Guard can poll for tag UIDs
type UIDReader interface {
	ReadUID(timeout time.Duration) (uid []byte, err error)
}

// Guard is a a door guard for NFC tags
type Guard struct {
	door   int32
	side   string
	reader UIDReader
	auth   auth.Authorizer
	gate   admitter.Admitter

	lastTag string

	// ReadTimeout is the time given to read a UID from the UIDReader, the
	// default is 100 milliseconds.
	ReadTimeout time.Duration
	// AuthTimeout id the overall time given for the authorization process, if
	// this time elapses before authorization is granted, then admission will
	// be denied. The default is 30 seconds.
	AuthTimeout time.Duration
	// CancelTimeout is the durition that a tag must be absent from the reader
	// before an in-progress auth operation is cancelled.
	CancelTimeout time.Duration
}

// New returs a new Guard, door is the id of this door, side of door is usually
// "A" or "B", reader is an instance of an NFC/RFID reader.
func New(door int32, side string, reader UIDReader, authority auth.Authorizer, gate admitter.Admitter) (*Guard, error) {
	return &Guard{
		door:          door,
		side:          side,
		reader:        reader,
		auth:          authority,
		gate:          gate,
		ReadTimeout:   defaultReadTimeoutMS * time.Millisecond,
		AuthTimeout:   defaultAuthTimeoutS * time.Second,
		CancelTimeout: defaultCanelTimeoutS * time.Second,
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
		g.lastTag = ""
		return nil
	}

	uid := hex.EncodeToString(rawUID)
	if uid == g.lastTag {
		return nil
	}
	g.lastTag = uid

	ctx := context.Background()
	ctx = context.WithValue(ctx, admitter.Door, g.door)
	ctx = context.WithValue(ctx, admitter.Side, g.side)
	ctx = context.WithValue(ctx, admitter.Type, guardType)
	ctx = context.WithValue(ctx, admitter.ID, uid)
	ctx, cancel := context.WithTimeout(ctx, g.AuthTimeout)

	g.gate.Interrogating(ctx, "Authorizing tag...")

	// If the admitee pulls their tag off the reader; cancel the context
	bgScan := make(chan struct{})
	defer func() { <-bgScan }()
	defer cancel()
	go func() {
		defer close(bgScan)

		lastSeen := time.Now()

		for {
			if ctx.Err() != nil {
				break
			}
			rawUID, err := g.reader.ReadUID(g.ReadTimeout)
			if err != nil || uid != hex.EncodeToString(rawUID) {
				// Either the tag is gone or there was a read error, show the
				// authentee some kindness and only cancel them if this
				// continues to be the case for a short time
				if !(time.Since(lastSeen) > g.CancelTimeout) {
					continue
				}
				cancel()
				return
			}
			lastSeen = time.Now()
		}
	}()

	allowed, msg, err := g.auth.Allowed(ctx, g.door, g.side, uid)
	if err != nil {
		if err := g.gate.Deny(ctx, "Error", err); err != nil {
			return fmt.Errorf("failed to deny access: %w", err)
		}
		return nil
	}
	if !allowed {
		if msg == "" {
			msg = "Access denied"
		}
		if err := g.gate.Deny(ctx, msg, admitter.AccessDenied); err != nil {
			return fmt.Errorf("failed to deny access: %w", err)
		}
		return nil
	}

	if msg == "" {
		msg = "Access granted"
	}
	if err := g.gate.Allow(ctx, msg); err != nil {
		return fmt.Errorf("failed to allow access: %w", err)
	}

	return nil
}
