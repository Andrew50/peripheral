package data
import (
	"fmt"
	"time"
)

func StringToTime(datetimeStr string) (time.Time, error) {
	var layout string

	if len(datetimeStr) == len("2023-01-01") {
		layout = "2006-01-02" // Date format
	} else if len(datetimeStr) == len("2023-01-01 11:17:30") {
		layout = "2006-01-02 15:04:05" // DateTime format
	} else {
		return time.Time{}, fmt.Errorf("unsupported datetime format: %s", datetimeStr)
	}

	parsedTime, err := time.Parse(layout, datetimeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse datetime: %w", err)
	}

	return parsedTime, nil
}
