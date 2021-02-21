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
	// db, err := sql.Open("mysql", "hmsdev:hmsdev@(hmsdev)/hms?charset=utf8mb4&collation=utf8mb4_unicode_ci")
	db, err := sql.Open("mysql", "hmsdev:hmsdev@(hmsdev)/hms")
	require.NoError(t, err)
	c := &Client{
		db: db,
	}

	res, err := c.CheckTagAccess(context.Background(), 1, DoorSideA, "9607166cf0e6342fb7f3")
	require.NoError(t, err)

	t.Logf("%+v", res)
	t.Fail()
}
