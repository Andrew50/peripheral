package utils

import (
	"fmt"
	"strconv"
	"time"
	"unicode"

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

// MillisFromUTCTime performs operations related to MillisFromUTCTime functionality.
func MillisFromUTCTime(timeObj time.Time) (models.Millis, error) {
	return models.Millis(timeObj), nil
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

// GetTimeFrame performs operations related to GetTimeFrame functionality.
func GetTimeFrame(timeframeString string) (int, string, string, int, error) {
	// if no identifer is passed, it means that it should be minute data
	lastChar := rune(timeframeString[len(timeframeString)-1])
	if unicode.IsDigit(lastChar) {
		num, err := strconv.Atoi(timeframeString)
		if err != nil {
			return 0, "", "", 0, err
		}
		return num, "minute", "m", 1, nil
	}
	// else, there is an identifier and not minute

	// add .toLower() or toUpper to not have to check two different cases
	identifier := string(timeframeString[len(timeframeString)-1])
	num, err := strconv.Atoi(timeframeString[:len(timeframeString)-1])
	if err != nil {
		return 0, "", "", 0, err
	}
	// add .toLower() or toUpper to not have to check two different cases
	if identifier == "s" {
		return num, "second", "s", 1, nil
	} else if identifier == "h" {
		return num, "hour", "m", 60, nil
	} else if identifier == "d" {
		return num, "day", "d", 1, nil
	} else if identifier == "w" {
		return num, "week", "d", 7, nil
	} else if identifier == "m" {
		return num, "month", "d", 30, nil
	} else if identifier == "y" {
		return num, "year", "d", 365, nil
	}
	return 0, "", "", 0, fmt.Errorf("incorrect timeframe passed")
}

// getStartOfTimeWindow calculates the start of a time window based on the given parameters
// nolint:unused
func getStartOfTimeWindow(timestamp time.Time, multiplier int, timespan string, extendedHours bool, location *time.Location) (time.Time, error) {

	timestamp = timestamp.In(location)

	switch timespan {
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
