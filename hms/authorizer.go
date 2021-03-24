package hms

import (
	"context"
	"time"
)

// Allowed makes hms into an nfc.Authorizer
func (c *Client) Allowed(ctx context.Context, door int32, side, id string) (allowed bool, message string, err error) {
	res, err := c.GatekeeperCheckRFID(ctx, door, side, id)
	if err != nil {
		return false, "", err
	}

	// As there is currently no door sensor, update the member location
	// directly after auth
	if res.AccessGranted {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			c.GatekeeperSetZone(ctx, res.MemberID, res.NewZoneID)
		}()
	}

	return res.AccessGranted, res.Message, nil
}
