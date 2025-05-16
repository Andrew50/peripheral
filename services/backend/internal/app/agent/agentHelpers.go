package agent

import (
	"backend/internal/data"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type DateToMSArgs struct {
	Date   string `json:"date"`
	Hour   int    `json:"hour,omitempty"`
	Minute int    `json:"minute,omitempty"`
	Second int    `json:"second,omitempty"`
}

func DateToMS(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DateToMSArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return 0, err
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, fmt.Errorf("unknown timezone %q", "America/New_York")
	}

	// Parse date
	base, err := time.ParseInLocation("2006-01-02", args.Date, loc)
	if err != nil {
		return 0, errors.New("date must be YYYY-MM-DD")
	}

	ts := time.Date(base.Year(), base.Month(), base.Day(),
		args.Hour, args.Minute, args.Second,
		0, loc).UnixMilli()

	return ts, nil

}
