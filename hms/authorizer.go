package hms

import (
	"context"

	"github.com/somakeit/door-controller3/nfc"
)

var _ nfc.Authorizer = &Client{}

// Allowed makes hms into an nfc.Authorizer
func (c *Client) Allowed(ctx context.Context, door int32, side, id string) (allowed bool, message string, err error) {
	res, err := c.GatekeeperCheckRFID(ctx, door, side, id)
	if err != nil {
		return false, "", err
	}
	return res.AccessGranted, res.Message, nil
}
