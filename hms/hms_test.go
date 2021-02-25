package hms

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	c, err := NewClient()
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestClientCheckTagAccess(t *testing.T) {
	db, err := sql.Open("mysql", "hmsdev:hmsdev@(hmsdev)/hms")
	require.NoError(t, err)
	c := &Client{
		db: db,
	}

	res, err := c.GatekeeperCheckRFID(context.Background(), 1, DoorSideA, "9607166cf0e6342fb7f3")
	require.NoError(t, err)

	t.Logf("%+v", res)
}
