package auth

import "context"

// Authorizer is an instance of the entity that says whether a given identifier
// is to be granted access or not. Errors from Allowed are non-fatal.
type Authorizer interface {
	Allowed(ctx context.Context, door int32, side, id string) (allowed bool, message string, err error)
}
