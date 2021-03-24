package hms

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestAuthorized(t *testing.T) {
	for name, test := range map[string]struct {
		door int32
		side string
		tag  string

		rows     []*sqlmock.Rows
		queryErr error

		want    bool
		wantMsg string
		wantErr bool
	}{
		"allowed": {
			door: 1,
			side: DoorSideB,
			tag:  "1f680",

			rows: []*sqlmock.Rows{
				sqlmock.NewRows([]string{
					"@message",
					"@memberName",
					"@lastSeen",
					"@accessGranted",
					"@newZoneID",
					"@memberID",
					"@spErr"}).
					AddRow(
						"Welcome back Bracken",
						"Bracken",
						"3h 14m 15s",
						int32(1),
						int32(5),
						int32(7),
						""),
			},

			want:    true,
			wantMsg: "Welcome back Bracken",
		},

		"notAllowed": {
			door: 1,
			side: DoorSideB,
			tag:  "1f4a9",

			rows: []*sqlmock.Rows{
				sqlmock.NewRows([]string{
					"@message",
					"@memberName",
					"@lastSeen",
					"@accessGranted",
					"@newZoneID",
					"@memberID",
					"@spErr"}).
					AddRow(
						"",
						"John",
						"3s",
						int32(0),
						int32(5),
						int32(99),
						""),
			},

			want: false,
		},

		"failedQuery": {
			door: 1,
			side: DoorSideB,
			tag:  "1f680",

			queryErr: errors.New("var not in scope"),

			want:    false,
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer require.NoError(t, mock.ExpectationsWereMet())
			defer db.Close()

			mock.ExpectExec(`CALL sp_gatekeeper_check_rfid\(\?, \?, \?, @message,
				@memberName, @lastSeen,	@accessGranted, @newZoneID, @memberID,
				@spErr\)`).
				WithArgs(test.tag, test.door, test.side).
				WillReturnResult(sqlmock.NewResult(0, 0)).
				WillReturnError(nil)
			mock.ExpectQuery(`SELECT @message, @memberName, @lastSeen, @accessGranted,
				@newZoneID, @memberID, @spErr`).
				WillReturnRows(test.rows...).
				WillReturnError(test.queryErr)

			c := &Client{db: db}
			got, msg, err := c.Allowed(context.Background(), test.door, test.side, test.tag)
			require.Equal(t, test.wantErr, err != nil, "wantErr=%t, err=%v", test.wantErr, err)
			require.Equal(t, test.want, got)
			require.Equal(t, test.wantMsg, msg)
		})
	}
}
