package utils

import (
	"fmt"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

var easternLocation *time.Location

func init() {
	var err error
	easternLocation, err = time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
}

// StringToTime performs operations related to StringToTime functionality.
func StringToTime(datetimeStr string) (time.Time, error) {
	layouts := []string{
		time.DateTime,
		time.DateOnly,
	}
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Time{}, fmt.Errorf("issue loading eastern location 31m9lffk")

	}
	for _, layout := range layouts {

		if dt, err := time.ParseInLocation(layout, datetimeStr, easternLocation); err == nil {

			return dt, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported datetime format: %s", datetimeStr)

	// aj old code
	/*var layout string

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

	return parsedTime, nil*/

}

// MillisFromDatetimeString performs operations related to MillisFromDatetimeString functionality.
func MillisFromDatetimeString(datetime string) (models.Millis, error) {
	layouts := []string{
		time.DateTime,
		time.DateOnly,
	}
	for _, layout := range layouts {
		easternTimeLocation, err := time.LoadLocation("America/New_York")
		if err != nil {
			return models.Millis(time.Now()), err
		}
		if dt, err := time.ParseInLocation(layout, datetime, easternTimeLocation); err == nil {
			return models.Millis(dt), nil
		}
	}
	return models.Millis(time.Now()), fmt.Errorf("212k invalid string datetime")
}

// NanosFromUTCTime performs operations related to NanosFromUTCTime functionality.
func NanosFromUTCTime(timeObj time.Time) (models.Nanos, error) {
	return models.Nanos(timeObj), nil
}

// NanosFromDatetimeString performs operations related to NanosFromDatetimeString functionality.
func NanosFromDatetimeString(datetime string) (models.Nanos, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.DateTime,
	}
	for _, layout := range layouts {
		if dt, err := time.Parse(layout, datetime); err == nil {
			easternTimeLocation, err := time.LoadLocation("America/New_York")
			if err != nil {
				return models.Nanos(time.Now()), fmt.Errorf("gw9ni2f3 %v", err)
			}
			return models.Nanos(dt.In(easternTimeLocation)), nil
		}
	}
	return models.Nanos(time.Now()), nil
}

// getStartOfTimeWindow calculates the start of a time window based on the given parameters
// nolint:unused
//
//lint:ignore U1000 kept for future time window calculations
func getStartOfTimeWindow(timestamp time.Time, multiplier int, timespan string, _ bool, location *time.Location) (time.Time, error) {
	switch timespan {
	case "minute":
		return timestamp.Add(time.Duration(-multiplier) * time.Minute).In(location), nil
	case "s":
		// Seconds
		duration := time.Duration(multiplier) * time.Second
		return timestamp.Truncate(duration), nil

	case "m":
		// Minutes
		duration := time.Duration(multiplier) * time.Minute
		return timestamp.Truncate(duration), nil

	case "h":
		// Hours
		duration := time.Duration(multiplier) * time.Hour
		return timestamp.Truncate(duration), nil

	case "d":
		// Days
		duration := time.Duration(multiplier*24) * time.Hour
		return timestamp.Truncate(duration), nil
	}
	return time.Time{}, fmt.Errorf("done")
}

// IsTimestampRegularHours performs operations related to IsTimestampRegularHours functionality.
func IsTimestampRegularHours(timestamp time.Time) bool {
	marketOpenTime := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 9, 30, 0, 0, easternLocation)
	marketCloseTime := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 16, 0, 0, 0, easternLocation)
	return !timestamp.Before(marketOpenTime) && timestamp.Before(marketCloseTime)
}
