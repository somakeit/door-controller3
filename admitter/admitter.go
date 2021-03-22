// Admitters are packages which impliment the consequences of granted or
// rejected authorization attmpets, such as; unlocking a strike, blinking an
// LED, or logging a message.
package admitter

import (
	"context"
	"errors"
)

type contextKey string

const (
	// ID is context key used to store the identifier of the atmittee (such as
	// a tag UID or PIN) on the context passed to the methods in Admitter. It
	// should not be shown to the admitee.
	ID contextKey = "uid"
	// Type is the context key used to store the kind of guard calling the
	// Admitter
	Type contextKey = "type"
	// Door is the context key used to store the door ID of the guard calling
	// the Admitter
	Door contextKey = "door"
	// Side is the context key used to store the door side of the guard calling
	// the Admitter
	Side contextKey = "side"
)

var (
	// AccessDenied is the reason error used if access was denied
	AccessDenied = errors.New("access denied")
)

// Admitter is the interface for consequences of admission attempts, it may be
// one output such as a strike or a mux of many; such as an LED and a strike.
type Admitter interface {
	// Interrogating is called once after an authorization attempt is started.
	// Implimentations should return immediately. The message is a user
	// presentable message. The context will contain the ID value and will be
	// cancelled as soon as the authorization attmpet finishes, regardless of
	// the result.
	Interrogating(ctx context.Context, message string)
	// Deny is called if an authorization attempt resulted in an explcity deny
	// or an error occured during authorization. The context will contain the
	// ID value and may already be cancelled. The reason will be AccessDenied
	// or if authorizarion failed, the actual error.
	Deny(ctx context.Context, message string, reason error) error
	// Allow is called if an authorization attempt was successful and the
	// admitee should be allowed in. The context will contain the ID value.
	Allow(ctx context.Context, message string) error
}

// Mux is a container for multiple Admitters, each is called sequentially in
// order.
type Mux []Admitter

func (m Mux) Interrogating(ctx context.Context, message string) {
	for _, a := range m {
		a.Interrogating(ctx, message)
	}
}

func (m Mux) Deny(ctx context.Context, message string, reason error) error {
	for _, a := range m {
		if err := a.Deny(ctx, message, reason); err != nil {
			return err
		}
	}
	return nil
}

func (m Mux) Allow(ctx context.Context, message string) error {
	for _, a := range m {
		if err := a.Allow(ctx, message); err != nil {
			return err
		}
	}
	return nil
}
