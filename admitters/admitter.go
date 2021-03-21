// Admitters are packages which impliment the consequences of granted or
// rejected authorization attmpets, such as; unlocking a strike, blinking an
// LED, or logging a message.
package admitters

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
	// admitee should be allowed in. The context will contain the ID value and
	// may already be cancelled.
	Allow(ctx context.Context, message string) error
}
