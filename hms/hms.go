// hms is a library of Go bindings for the hms2 database stored procedures
package hms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Logger can be used to interface any logger to this package, by default
// it discards all logs
var Logger ContextLogger = logDiscarder{}

// ContextLogger is an interface which allows you to use any logger and include
// context filds.
type ContextLogger interface {
	Warn(ctx context.Context, args ...interface{})
	Warnf(ctx context.Context, args ...interface{})
}

type logDiscarder struct{}

func (logDiscarder) Warn(context.Context, ...interface{})  {}
func (logDiscarder) Warnf(context.Context, ...interface{}) {}

// Client provides methods for interfacing with the HMS2 databse
type Client struct {
	db *sql.DB
	// because stored procedures are executed before their results are selected
	// we must make sure no other procedures are called before those results
	// are selected.
	scope sync.Mutex
}

// NewClient returns a new HMS2 database Client, db must be an opened hms2 sql
// database
func NewClient(db *sql.DB) (*Client, error) {
	return &Client{
		db: db,
	}, nil
}

// GatekeeperCheckRFID checks an rfid serial is valid and if access is allowed.
// Then logs an entry in the access log (either granted or denied). Then
// returns whether access was granted and an approprite unlock text in
// GatekeeperCheckResult if it is.
func (c *Client) GatekeeperCheckRFID(ctx context.Context, door int32, side, tag string) (GatekeeperCheckResult, error) {
	var result *sql.Rows
	if err := func() error {
		c.scope.Lock()
		defer c.scope.Unlock()

		_, err := c.db.ExecContext(
			ctx,
			`CALL sp_gatekeeper_check_rfid(?, ?, ?, @message, @memberName, @lastSeen,
				@accessGranted, @newZoneID, @memberID, @spErr)`,
			tag,
			door,
			side,
		)
		if err != nil {
			return fmt.Errorf("failed to execute sp: %w", err)
		}
		result, err = c.db.QueryContext(ctx, `SELECT @message, @memberName, @lastSeen,
			@accessGranted, @newZoneID, @memberID, @spErr`)
		if err != nil {
			return fmt.Errorf("failed to select sp result: %w", err)
		}

		return nil
	}(); err != nil {
		return GatekeeperCheckResult{}, err
	}

	if !result.Next() {
		return GatekeeperCheckResult{}, errors.New("no sp result")
	}

	var (
		message       sql.NullString
		memberName    sql.NullString
		lastSeen      sql.NullString
		accessGranted sql.NullInt32
		newZoneID     sql.NullInt32
		memberID      sql.NullInt32
		spErr         sql.NullString
	)
	err := result.Scan(&message, &memberName, &lastSeen, &accessGranted, &newZoneID, &memberID, &spErr)
	if err != nil {
		return GatekeeperCheckResult{}, fmt.Errorf("error scanning sp result: %w", err)
	}

	if spErr.String != "" {
		return GatekeeperCheckResult{}, fmt.Errorf("sp failed: %s", spErr.String)
	}

	return GatekeeperCheckResult{
		// This check on Valid is redundant but I do not want any surprises
		AccessGranted: accessGranted.Valid && accessGranted.Int32 == granted,
		LastSeen:      parseDuration(ctx, lastSeen),
		Message:       message.String,
		MemberID:      memberID.Int32,
		MemberName:    memberName.String,
		NewZoneID:     newZoneID.Int32,
	}, nil
}

// GatekeeperSetZone updates the zone_occupancy table with the new zone the of
// member, and log an entry to zone_occupancy_log to record what time the
// previous zone was entered/left
func (c *Client) GatekeeperSetZone(ctx context.Context, memberID, newZoneID int32) {
	if _, err := c.db.Exec("CALL sp_gatekeeper_set_zone(?, ?)", memberID, newZoneID); err != nil {
		Logger.Warnf(ctx, "Failed to set mebmer %d to zone %d: %s", memberID, newZoneID, err)
	}
}

// GatekeeperCheckPIN checks a pin is valid and returns an approprite unlock
// text if it is. If the PIN is found and is set to enroll then the last card
// read will be registered (if within timeout). If registation is successfull,
// the pin is considered invalid. In all cases an entry is made in the access
// log.
func (c *Client) GatekeeperCheckPIN(ctx context.Context, door int32, side, pin string) (GatekeeperCheckResult, error) {
	var result *sql.Rows
	if err := func() error {
		c.scope.Lock()
		defer c.scope.Unlock()

		_, err := c.db.ExecContext(
			ctx,
			`CALL sp_gatekeeper_check_pin(?, ?, ?, @memberID, @newZoneID, @message,
				@memberName, @spErr)`,
			pin,
			door,
			side,
		)
		if err != nil {
			return fmt.Errorf("failed to execute sp: %w", err)
		}

		result, err = c.db.QueryContext(ctx, `SELECT @memberID, @newZoneID, @message,
			@memberName, @spErr`)
		if err != nil {
			return fmt.Errorf("failed to select sp result: %w", err)
		}

		return nil
	}(); err != nil {
		return GatekeeperCheckResult{}, err
	}

	if !result.Next() {
		return GatekeeperCheckResult{}, errors.New("no sp result")
	}

	var (
		memberID   sql.NullInt32
		newZoneID  sql.NullInt32
		message    sql.NullString
		memberName sql.NullString
		spErr      sql.NullString
	)
	if err := result.Scan(&memberID, &newZoneID, &message, &memberName, &spErr); err != nil {
		return GatekeeperCheckResult{}, fmt.Errorf("error scanning sp result: %w", err)
	}

	if spErr.String != "" {
		return GatekeeperCheckResult{}, fmt.Errorf("sp failed: %s", spErr.String)
	}

	return GatekeeperCheckResult{
		// No explicit access_granted field on this sp
		AccessGranted: message.String != "",
		// No last_seen field on this sp
		Message:    message.String,
		MemberID:   memberID.Int32,
		MemberName: memberName.String,
		NewZoneID:  newZoneID.Int32,
	}, nil
}

const (
	// DoorSideA is usually outide
	DoorSideA = "A"
	// DoorSideB is usually inside
	DoorSideB = "B"

	// access granted is 1 in the stored procedure logic
	granted = 1
)

// GatekeeperCheckResult is the result of checking tag access at a door
type GatekeeperCheckResult struct {
	// AccessGranted indicates whether the door should be opened
	AccessGranted bool
	// LastSeen is the time since the tag's owner was seen
	LastSeen time.Duration
	// Message is an appropriate display message and is only set if
	// AccessGranted is true
	Message string
	// MemberID is the member that owns the tag
	MemberID int32
	// MemberName is the username of the member
	MemberName string
	// NewZoneID is the zone the member would be moving into
	NewZoneID int32
}

func parseDuration(ctx context.Context, t sql.NullString) time.Duration {
	if !t.Valid {
		return 0
	}

	d, err := time.ParseDuration(strings.ReplaceAll(t.String, " ", ""))
	if err != nil {
		Logger.Warn(ctx, "Failed to parse duration: ", err)
		return 0
	}

	return d
}
