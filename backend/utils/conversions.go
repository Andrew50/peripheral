package utils

import (
	"fmt"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

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

func MillisFromUTCTime(timeObj time.Time) (models.Millis, error) {
	return models.Millis(timeObj), nil
}
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
