package hms

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestClientCheckTagAccess(t *testing.T) {
	db, err := sql.Open("mysql", "hmsdev:hmsdev@(hmsdev)/hms")
	require.NoError(t, err)
	c, err := NewClient(db)
	require.NoError(t, err)

	res, err := c.GatekeeperCheckRFID(context.Background(), 1, DoorSideA, "9607166cf0e6342fb7f3")
	require.NoError(t, err)
	if res.AccessGranted {
		c.GatekeeperSetZone(res.MemberID, res.NewZoneID)
	}

	t.Logf("%+v", res)
}
