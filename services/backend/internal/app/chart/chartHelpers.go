package chart

import (
	"fmt"
	"strconv"
	"time"
	"unicode"

	"github.com/polygon-io/client-go/rest/models"
)

// MillisFromUTCTime performs operations related to MillisFromUTCTime functionality.
func MillisFromUTCTime(timeObj time.Time) (models.Millis, error) {
	return models.Millis(timeObj), nil
}

// TimespanStringToDuration converts a timespan string to a time.Duration
func TimespanStringToDuration(timespan string) time.Duration {
	switch timespan {
	case "minute":
		return time.Minute
	case "hour":
		return time.Hour
	case "day":
		return 24 * time.Hour
	case "week":
		return 7 * 24 * time.Hour
	case "month":
		return 30 * 24 * time.Hour
	case "quarter":
		return 3 * 30 * 24 * time.Hour
	case "year":
		return 365 * 24 * time.Hour
	default:
		return time.Minute // Default to minute if unknown
	}
}

// GetRequestStartEndTime performs operations related to GetRequestStartEndTime functionality.
func GetRequestStartEndTime(
	lowerDate time.Time,
	upperDate time.Time,
	direction string,
	timespan string,
	multiplier int,
	bars int,
) (models.Millis, models.Millis, error) {

	overestimate := 2.0
	// Create a default return value in case of error
	badReturn, _ := MillisFromUTCTime(time.Now())
	if direction != "backward" && direction != "forward" {
		return badReturn, badReturn, fmt.Errorf("invalid direction; must be 'back' or 'forward'")
	}
	barDuration := TimespanStringToDuration(timespan) * time.Duration(multiplier)
	totalDuration := barDuration * time.Duration(int(float64(bars)*overestimate))
	tradingMinutesPerDay := 960.0 // 16 hours * 60 minutes
	tradingDurationPerDay := time.Duration(int(tradingMinutesPerDay)) * time.Minute
	totalTradingDays := totalDuration / tradingDurationPerDay
	totalTradingDays += 6 //overestimate cause weekends
	var queryStartTime, queryEndTime time.Time
	if direction == "backward" {
		queryStartTime = upperDate.AddDate(0, 0, -int(totalTradingDays))
		if queryStartTime.Before(lowerDate) {
			queryStartTime = lowerDate
		}
		queryEndTime = upperDate
		queryStartTime = time.Date(queryStartTime.Year(), queryStartTime.Month(), queryStartTime.Day(), 0, 0, 0, 0, queryStartTime.Location())
	} else {
		queryEndTime = lowerDate.AddDate(0, 0, int(totalTradingDays))
		if queryEndTime.After(upperDate) {
			queryEndTime = upperDate
		}
		queryStartTime = lowerDate
	}
	startMillis, err := MillisFromUTCTime(queryStartTime)
	if err != nil {
		return badReturn, badReturn, err
	}
	endMillis, err := MillisFromUTCTime(queryEndTime)
	if err != nil {
		return badReturn, badReturn, err
	}
	return startMillis, endMillis, nil
}

// GetTimeframeInSeconds performs operations related to GetTimeframeInSeconds functionality.
func GetTimeframeInSeconds(multiplier int, timeframe string) int64 {
	if timeframe == "hour" {
		return 60 * 60 * int64(multiplier)
	}
	if timeframe == "minute" {
		return 60 * int64(multiplier)
	}
	if timeframe == "second" {
		return int64(multiplier)
	}
	if timeframe == "day" {
		return 24 * 60 * 60 * int64(multiplier)
	}
	if timeframe == "week" {
		return 7 * 24 * 60 * 60 * int64(multiplier)
	}

	return 0
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
	}
	if identifier == "h" {
		return num, "hour", "m", 60, nil
	}
	if identifier == "d" {
		return num, "day", "d", 1, nil
	}
	if identifier == "w" {
		return num, "week", "d", 7, nil
	}
	if identifier == "m" {
		return num, "month", "d", 30, nil
	}
	if identifier == "y" {
		return num, "year", "d", 365, nil
	}
	return 0, "", "", 0, fmt.Errorf("incorrect timeframe passed")
}

// GetReferenceStartTimeForMonths performs operations related to GetReferenceStartTimeForMonths functionality.
func GetReferenceStartTimeForMonths(timestamp int64, _ int, easternLocation *time.Location) int64 {
	utcTime := time.Unix(0, timestamp*int64(time.Millisecond)).UTC()
	nyTime := utcTime.In(easternLocation)

	// Reference date: September 1, 2003, in New York time
	referenceDate := time.Date(2003, time.September, 1, 0, 0, 0, 0, easternLocation)
	// Calculate the difference in months between the current time and the reference date
	elapsedTimeInMonths := (nyTime.Year()-referenceDate.Year())*12 + int(nyTime.Month()) - int(referenceDate.Month())
	// Calculate the start date for the current bar
	candleStartTime := referenceDate.AddDate(0, elapsedTimeInMonths, 0)
	// Convert the start time to Unix time in milliseconds and return it
	return candleStartTime.UnixMilli()
}

// GetReferenceStartTimeForWeeks performs operations related to GetReferenceStartTimeForWeeks functionality.
func GetReferenceStartTimeForWeeks(timestamp int64, multiplier int, easternLocation *time.Location) int64 {
	utcTime := time.Unix(0, timestamp*int64(time.Millisecond)).UTC()
	nyTime := utcTime.In(easternLocation)
	// Reference date: September 7, 2003, in New York time (Sunday of that week)
	referenceDate := time.Date(2003, time.September, 7, 0, 0, 0, 0, easternLocation)
	// Calculate the difference in weeks between the current time and the reference date
	elapsedTimeInWeeks := int(nyTime.Sub(referenceDate).Hours() / (24 * 7))
	// Calculate the number of full bars (based on the multiplier, which is the timeframe in weeks)
	numFullBars := elapsedTimeInWeeks / multiplier
	// Calculate the start date for the current bar
	candleStartTime := referenceDate.AddDate(0, 0, numFullBars*multiplier*7)
	// Convert the start time to Unix time in milliseconds and return it
	return candleStartTime.UnixMilli()
}

// GetReferenceStartTimeForDays performs operations related to GetReferenceStartTimeForDays functionality.
func GetReferenceStartTimeForDays(timestamp int64, multiplier int, easternLocation *time.Location) int64 {
	utcTime := time.Unix(0, timestamp*int64(time.Millisecond)).UTC()
	nyTime := utcTime.In(easternLocation)
	var referenceDate time.Time
	if multiplier == 1 {
		referenceDate = time.Date(2003, time.September, 10, 0, 0, 0, 0, easternLocation)
	} else {
		referenceDate = time.Date(2003, time.September, 9, 0, 0, 0, 0, easternLocation)
	}
	elapsedTimeInDays := int(nyTime.Sub(referenceDate).Hours() / 24)
	numFullBars := elapsedTimeInDays / multiplier
	candleStartTime := referenceDate.AddDate(0, 0, numFullBars*multiplier)
	return candleStartTime.UnixMilli()
}

// GetReferenceStartTime performs operations related to GetReferenceStartTime functionality.
func GetReferenceStartTime(timestamp int64, extendedHours bool, easternLocation *time.Location) int64 {
	utcTime := time.Unix(0, timestamp*int64(time.Millisecond)).UTC()
	nyTime := utcTime.In(easternLocation)
	year, month, day := nyTime.Date()
	var referenceTime time.Time
	if extendedHours {
		// If extendedHours is true, set reference time to 4:00 AM EST/EDT
		referenceTime = time.Date(year, month, day, 4, 0, 0, 0, easternLocation)
	} else {
		// If extendedHours is false, set reference time to 9:30 AM EST/EDT
		referenceTime = time.Date(year, month, day, 9, 30, 0, 0, easternLocation)
	}

	// Step 5: Convert the reference time back to UTC
	referenceUTC := referenceTime.UTC()

	// Step 6: Convert the reference UTC time back to Unix timestamp in milliseconds
	referenceTimestamp := referenceUTC.Unix()*1000 + int64(referenceUTC.Nanosecond())/int64(time.Millisecond)

	return referenceTimestamp
}
