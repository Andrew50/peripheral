package utils

import (
	"fmt"
	"strconv"
	"time"
	"unicode"
)

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
