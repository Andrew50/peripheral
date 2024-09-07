package data

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
	for _, layout := range layouts {
		if dt, err := time.Parse(layout, datetimeStr); err == nil {
			easternTimeLocation, tzErr := time.LoadLocation("America/New_York")
			if tzErr != nil {
				return time.Time{}, fmt.Errorf("Failed to load EST timezone: %w", err)
			}
			return dt.In(easternTimeLocation), nil
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
func MillisFromDatetimeString(datetime string) (models.Millis, error){
	layouts := []string{
		time.DateTime,
		time.DateOnly,
	}
	for _, layout := range layouts {
		if dt, err := time.Parse(layout, datetime); err == nil {
			easternTimeLocation, err := time.LoadLocation("America/New_York")
			if err != nil {
                return models.Millis(time.Now()), err
			}
			return models.Millis(dt.In(easternTimeLocation)), nil
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
                return models.Nanos(time.Now()), fmt.Errorf("gw9ni2f3 %v",err)
			}
			return models.Nanos(dt.In(easternTimeLocation)), nil
		}
	}
	return models.Nanos(time.Now()), nil
}
