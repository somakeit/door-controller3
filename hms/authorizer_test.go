package hms

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestAuthorized(t *testing.T) {
	for name, test := range map[string]struct {
		door int32
		side string
		tag  string

		member   int
		rows     []*sqlmock.Rows
		queryErr error

		want          bool
		wantMsg       string
		wantErr       bool
		wantLocUpdate bool
	}{
		"allowed": {
			door: 1,
			side: DoorSideB,
			tag:  "1f680",

			member: 7,
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

			want:          true,
			wantMsg:       "Welcome back Bracken",
			wantLocUpdate: true,
		},

		"notAllowed": {
			door: 1,
			side: DoorSideB,
			tag:  "1f4a9",

			member: 99,
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

			want:          false,
			wantLocUpdate: false,
		},

		"failedQuery": {
			door: 1,
			side: DoorSideB,
			tag:  "1f680",

			queryErr: errors.New("var not in scope"),

			want:          false,
			wantErr:       true,
			wantLocUpdate: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { require.NoError(t, mock.ExpectationsWereMet()) }()
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
			if test.wantLocUpdate {
				mock.ExpectExec("CALL sp_gatekeeper_set_zone").
					WithArgs(test.member, 5).
					WillReturnResult(sqlmock.NewResult(0, 0)).
					WillReturnError(nil)
			}

			c := &Client{db: db}
			got, msg, err := c.Allowed(context.Background(), test.door, test.side, test.tag)
			require.Equal(t, test.wantErr, err != nil, "wantErr=%t, err=%v", test.wantErr, err)
			require.Equal(t, test.want, got)
			require.Equal(t, test.wantMsg, msg)
			time.Sleep(50 * time.Millisecond)
		})
	}
}
