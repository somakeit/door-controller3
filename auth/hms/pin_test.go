package hms

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestCheckPIN(t *testing.T) {
	for name, test := range map[string]struct {
		door int32
		side string
		pin  string

		rows              []*sqlmock.Rows
		execErr, queryErr error

		want    string
		wantErr string
	}{
		"allowed": {
			door: 1,
			side: DoorSideB,
			pin:  "0248",

			rows: []*sqlmock.Rows{
				sqlmock.NewRows([]string{
					"@memberID",
					"@newZoneID",
					"@message",
					"@memberName",
					"@spErr",
				}).
					AddRow(
						int32(7),
						int32(5),
						"Welcome back Bracken",
						"Bracken",
						""),
			},

			want: "Valid pin for Bracken (id=7): Welcome back Bracken",
		},

		"notAllowed": {
			door: 1,
			side: DoorSideB,
			pin:  "1234",

			rows: []*sqlmock.Rows{
				sqlmock.NewRows([]string{
					"@memberID",
					"@newZoneID",
					"@message",
					"@memberName",
					"@spErr",
				}).
					AddRow(
						int32(99),
						int32(5),
						"",
						"John",
						""),
			},

			want: "Invalid pin",
		},

		"failedQuery": {
			door: 1,
			side: DoorSideB,
			pin:  "0248",

			queryErr: errors.New("var not in scope"),

			want:    "",
			wantErr: "failed to select sp result: var not in scope",
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { require.NoError(t, mock.ExpectationsWereMet()) }()
			defer db.Close()

			mock.ExpectExec(`CALL sp_gatekeeper_check_pin\(\?, \?, \?, @memberID,
				@newZoneID, @message, @memberName, @spErr\)`).
				WithArgs(test.pin, test.door, test.side).
				WillReturnResult(sqlmock.NewResult(0, 0)).
				WillReturnError(test.execErr)
			if test.execErr == nil {
				mock.ExpectQuery(`SELECT @memberID,	@newZoneID, @message, @memberName,
				@spErr`).
					WillReturnRows(test.rows...).
					WillReturnError(test.queryErr)
			}

			c := &Client{db: db}
			got, err := c.CheckPIN(context.Background(), test.door,
				test.side, test.pin)
			if test.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
			}
			require.Equal(t, test.want, got)
		})
	}
}
