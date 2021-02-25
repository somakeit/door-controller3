package hms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Client provides methods for interfacing with the HMS2 databse
type Client struct {
	db *sql.DB
	// because stored procedures are executed before their results are selected
	// we must make sure no other procedures are called before those results
	// are selected.
	scope sync.Mutex
}

// NewClient returns a new HMS2 database Client, scripts is a path to the
// directory holding the sql scripts.
func NewClient() (*Client, error) {
	return &Client{}, nil
}

// GatekeeperCheckRFID takes a door ID, a side and a tag serial number then
// returns GatekeeperCheckRFIDResult including whether access is allowed.
func (c *Client) GatekeeperCheckRFID(ctx context.Context, door int, side DoorSide, tag string) (GatekeeperCheckRFIDResult, error) {
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
		return GatekeeperCheckRFIDResult{}, err
	}

	if !result.Next() {
		return GatekeeperCheckRFIDResult{}, errors.New("no sp result")
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
		return GatekeeperCheckRFIDResult{}, fmt.Errorf("error scanning sp result: %w", err)
	}

	if spErr.String != "" {
		return GatekeeperCheckRFIDResult{}, fmt.Errorf("sp failed: %s", spErr.String)
	}

	return GatekeeperCheckRFIDResult{
		// This check on Valid is redundant but I do not want any surprises
		AccessGranted: accessGranted.Valid && accessGranted.Int32 == granted,
		LastSeen:      parseDuration(lastSeen),
		Message:       message.String,
		MemberID:      memberID.Int32,
		MemberName:    memberName.String,
		NewZoneID:     newZoneID.Int32,
	}, nil
}

// DoorSide is the side of a door, valid valued are DoorSideA and DoorSideB
type DoorSide string

const (
	// DoorSideA is usually outide
	DoorSideA DoorSide = "A"
	// DoorSideB is usually inside
	DoorSideB DoorSide = "B"

	// access granted is 1 in the stored procedure logic
	granted = 1
	denied  = 0
)

// GatekeeperCheckRFIDResult is the result of checking tag access at a door
type GatekeeperCheckRFIDResult struct {
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

func parseDuration(t sql.NullString) time.Duration {
	if !t.Valid {
		return 0
	}

	d, err := time.ParseDuration(strings.ReplaceAll(t.String, " ", ""))
	if err != nil {
		log.Print("Failed to parse duration: ", err)
		return 0
	}

	return d
}
