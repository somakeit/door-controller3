package hms

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestGateKeeperCheckRFID(t *testing.T) {
	for name, test := range map[string]struct {
		door int32
		side string
		tag  string

		rows              []*sqlmock.Rows
		execErr, queryErr error

		want    *GatekeeperCheckResult
		wantErr string
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

			want: &GatekeeperCheckResult{
				AccessGranted: true,
				LastSeen:      time.Duration(3*time.Hour + 14*time.Minute + 15*time.Second),
				Message:       "Welcome back Bracken",
				MemberID:      7,
				MemberName:    "Bracken",
				NewZoneID:     5,
			},
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

			want: &GatekeeperCheckResult{
				AccessGranted: false,
				LastSeen:      time.Duration(3 * time.Second),
				Message:       "",
				MemberID:      99,
				MemberName:    "John",
				NewZoneID:     5,
			},
		},

		"unknown": {
			door: 1,
			side: DoorSideB,
			tag:  "8008135",

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
						nil,
						nil,
						int32(0),
						int32(5),
						nil,
						""),
			},

			want: &GatekeeperCheckResult{
				AccessGranted: false,
				LastSeen:      time.Duration(0),
				Message:       "",
				MemberID:      0,
				MemberName:    "",
				NewZoneID:     5,
			},
		},

		"invalidTime": {
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
						"1234234", // not valid
						int32(1),
						int32(5),
						int32(7),
						""),
			},

			want: &GatekeeperCheckResult{
				AccessGranted: true,
				LastSeen:      time.Duration(0),
				Message:       "Welcome back Bracken",
				MemberID:      7,
				MemberName:    "Bracken",
				NewZoneID:     5,
			},
		},

		"failedExec": {
			door: 1,
			side: DoorSideB,
			tag:  "1f680",

			execErr: errors.New("no such sp"),

			want:    &GatekeeperCheckResult{},
			wantErr: "failed to execute sp: no such sp",
		},

		"failedQuery": {
			door: 1,
			side: DoorSideB,
			tag:  "1f680",

			queryErr: errors.New("var not in scope"),

			want:    &GatekeeperCheckResult{},
			wantErr: "failed to select sp result: var not in scope",
		},

		"noResults": {
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
					"@spErr"}),
			},

			want:    &GatekeeperCheckResult{},
			wantErr: "no sp result",
		},

		"scanError": {
			door: 1,
			side: DoorSideB,
			tag:  "1f680",

			rows: []*sqlmock.Rows{
				sqlmock.NewRows([]string{
					"@weird"}).AddRow("things"),
			},

			want:    &GatekeeperCheckResult{},
			wantErr: "error scanning sp result: ",
		},

		"spErr": {
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
						"something went wrong"),
			},

			want:    &GatekeeperCheckResult{},
			wantErr: "sp failed: something went wrong",
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
				WillReturnError(test.execErr)
			if test.execErr == nil {
				mock.ExpectQuery(`SELECT @message, @memberName, @lastSeen, @accessGranted,
				@newZoneID, @memberID, @spErr`).
					WillReturnRows(test.rows...).
					WillReturnError(test.queryErr)
			}

			c := &Client{db: db}
			got, err := c.GatekeeperCheckRFID(context.Background(), test.door,
				test.side, test.tag)
			if test.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
			}
			if test.want != nil {
				require.Equal(t, *test.want, got)
			}
		})
	}
}

func TestGateKeeperCheckPIN(t *testing.T) {
	for name, test := range map[string]struct {
		door int32
		side string
		pin  string

		rows              []*sqlmock.Rows
		execErr, queryErr error

		want    *GatekeeperCheckResult
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

			want: &GatekeeperCheckResult{
				AccessGranted: true,
				Message:       "Welcome back Bracken",
				MemberID:      7,
				MemberName:    "Bracken",
				NewZoneID:     5,
			},
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

			want: &GatekeeperCheckResult{
				AccessGranted: false,
				Message:       "",
				MemberID:      99,
				MemberName:    "John",
				NewZoneID:     5,
			},
		},

		"unknown": {
			door: 1,
			side: DoorSideB,
			pin:  "5678",

			rows: []*sqlmock.Rows{
				sqlmock.NewRows([]string{
					"@memberID",
					"@newZoneID",
					"@message",
					"@memberName",
					"@spErr",
				}).
					AddRow(
						nil,
						int32(5),
						"",
						nil,
						""),
			},

			want: &GatekeeperCheckResult{
				AccessGranted: false,
				Message:       "",
				MemberID:      0,
				MemberName:    "",
				NewZoneID:     5,
			},
		},

		"failedExec": {
			door: 1,
			side: DoorSideB,
			pin:  "0248",

			execErr: errors.New("no such sp"),

			want:    &GatekeeperCheckResult{},
			wantErr: "failed to execute sp: no such sp",
		},

		"failedQuery": {
			door: 1,
			side: DoorSideB,
			pin:  "0248",

			queryErr: errors.New("var not in scope"),

			want:    &GatekeeperCheckResult{},
			wantErr: "failed to select sp result: var not in scope",
		},

		"noResults": {
			door: 1,
			side: DoorSideB,
			pin:  "0248",

			rows: []*sqlmock.Rows{
				sqlmock.NewRows([]string{
					"@message",
					"@memberName",
					"@lastSeen",
					"@accessGranted",
					"@newZoneID",
					"@memberID",
					"@spErr"}),
			},

			want:    &GatekeeperCheckResult{},
			wantErr: "no sp result",
		},

		"scanError": {
			door: 1,
			side: DoorSideB,
			pin:  "0248",

			rows: []*sqlmock.Rows{
				sqlmock.NewRows([]string{
					"@weird"}).AddRow("things"),
			},

			want:    &GatekeeperCheckResult{},
			wantErr: "error scanning sp result: ",
		},

		"spErr": {
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
						"oh no"),
			},

			want:    &GatekeeperCheckResult{},
			wantErr: "sp failed: oh no",
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
			got, err := c.GatekeeperCheckPIN(context.Background(), test.door,
				test.side, test.pin)
			if test.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
			}
			if test.want != nil {
				require.Equal(t, *test.want, got)
			}
		})
	}
}

func TestGatekeeperSetZone(t *testing.T) {
	for name, test := range map[string]struct {
		member, zone int32
		err          error
	}{
		"success": {1, 2, nil},
		"failure": {3, 4, errors.New("oops")},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err, "this error is surely impossible? %s", err)
			defer func() { require.NoError(t, mock.ExpectationsWereMet()) }()
			defer db.Close()

			mock.ExpectExec("CALL sp_gatekeeper_set_zone").
				WithArgs(test.member, test.zone).
				WillReturnResult(sqlmock.NewResult(0, 0)).
				WillReturnError(test.err)

			c, err := NewClient(db)
			require.NoError(t, err)
			c.GatekeeperSetZone(context.Background(), test.member, test.zone)
		})
	}
}

func TestGatekeeperCheckRFIDReal(t *testing.T) {
	if _, err := net.LookupHost("hmsdev"); err != nil {
		t.Skip("No database found:", err)
	}
	db, err := sql.Open("mysql", "hmsdev:hmsdev@(hmsdev)/hms")
	require.NoError(t, err)

	c, err := NewClient(db)
	require.NoError(t, err)

	res, err := c.GatekeeperCheckRFID(context.Background(), 1, DoorSideA, "9607166cf0e6342fb7f3")
	require.NoError(t, err)
	if res.AccessGranted {
		c.GatekeeperSetZone(context.Background(), res.MemberID, res.NewZoneID)
	}

	t.Logf("%+v", res)
}
