package agent

import (
	"backend/internal/data"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type DateToMSItem struct {
	Date   string `json:"date"`
	Hour   int    `json:"hour,omitempty"`
	Minute int    `json:"minute,omitempty"`
	Second int    `json:"second,omitempty"`
}

type DateToMSArgs struct {
	Dates []DateToMSItem `json:"dates"`
}

func DateToMS(_ *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args DateToMSArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}

	if len(args.Dates) == 0 {
		return nil, errors.New("at least one date is required")
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("unknown timezone %q", "America/New_York")
	}

	results := make([]int64, len(args.Dates))

	for i, dateItem := range args.Dates {
		// Parse date
		base, err := time.ParseInLocation("2006-01-02", dateItem.Date, loc)
		if err != nil {
			return nil, fmt.Errorf("date at index %d must be YYYY-MM-DD", i)
		}

		ts := time.Date(base.Year(), base.Month(), base.Day(),
			dateItem.Hour, dateItem.Minute, dateItem.Second,
			0, loc).UnixMilli()

		results[i] = ts
	}

	return results, nil
}
