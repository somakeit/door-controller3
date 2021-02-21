package hms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Client provides methods for interfacing with the HMS2 databse
type Client struct {
	db *sql.DB
}

// NewClient returns a new HMS2 database Client, scripts is a path to the
// directory holding the sql scripts.
func NewClient() (*Client, error) {
	return &Client{}, nil
}

// DoorSide is the side of a door
type DoorSide string

const (
	// DoorSideA is usually outise
	DoorSideA DoorSide = "A"
	// DoorSideB is usually inside
	DoorSideB DoorSide = "B"
)

// TagAccessResult is the result of checking tag access at a door
type TagAccessResult struct {
	// AccessGranted indicates whether the door should be opened
	AccessGranted bool
	// Message is an appropriate display message and is only set if
	// AccessGranted is true
	LastSeen time.Time
	// MemberID is the member that owns the tag
	Message string
	// LastSeen is the last time the tag's owner was seen
	MemberID int
	// MemberName is the username of the member
	MemberName string
	// NewZoneID is the zone the member would be moving into
	NewZoneID int
}

// CheckTagAccess takes a door ID, a side and a tag serial number then returns
// a TagAccessResult including whether access is allowed.
func (c *Client) CheckTagAccess(ctx context.Context, door int, side DoorSide, tag string) (TagAccessResult, error) {
	// var message, memberName, lastSeen, accessGranted, newZoneID, memberID, spErr interface{}
	var (
		memberID int
		spErr    string
	)

	// _, err := c.db.ExecContext(
	// 	ctx,
	// 	"CALL sp_gatekeeper_check_rfid(?, ?, ?, @message, @memberName, @lastSeen, @accessGranted, @newZoneID, @memberID, @spErr)",
	// 	tag,
	// 	door,
	// 	side,
	// )
	_, err := c.db.ExecContext(
		ctx,
		"CALL sp_check_rfid(?, @member_id, @sp_err)",
		tag,
	)
	if err != nil {
		return TagAccessResult{}, fmt.Errorf("failed to execute sp: %w", err)
	}
	// result, err := c.db.QueryContext(ctx, "SELECT @message, @memberName, @lastSeen, @accessGranted, @newZoneID, @memberID, @spErr")
	result, err := c.db.QueryContext(ctx, "SELECT @member_id, @sp_err")
	if err != nil {
		return TagAccessResult{}, fmt.Errorf("failed to select result from sp: %w", err)
	}

	if !result.Next() {
		return TagAccessResult{}, errors.New("no result from sp")
	}
	// TODO handle NULL for any field
	// err = result.Scan(&message, &memberName, &lastSeen, &accessGranted, &newZoneID, &memberID, &spErr)
	err = result.Scan(&memberID, &spErr)
	if err != nil {
		return TagAccessResult{}, err
	}

	// if spErrString, ok := spErr.(string); ok {
	// 	fmt.Errorf("sp failed: %s", spErrString)
	// }
	if spErr != "" {
		return TagAccessResult{}, fmt.Errorf("sp failed: %s", spErr)
	}

	return TagAccessResult{
		MemberID: memberID,
	}, nil
}
