package hms

import (
	"context"
	"fmt"
)

// CheckPIN is an adapter for the door, it wraps GateKeeperCheckPIN but returns
// just a string.
func (c *Client) CheckPIN(ctx context.Context, door int32, side, pin string) (string, error) {
	res, err := c.GatekeeperCheckPIN(ctx, door, side, pin)
	if err != nil {
		return "", err
	}
	if !res.AccessGranted {
		return "Invalid pin", nil
	}
	return fmt.Sprintf("Valid pin for %s (id=%d): %s", res.MemberName, res.MemberID, res.Message), nil
}
