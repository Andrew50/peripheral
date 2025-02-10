package alerts

import (
	"backend/socket"
	"time"
)

func isORBreakout(sd *socket.SecurityData) bool {
	// 1. Lock second data
	sd.SecondDataExtended.Mutex.RLock()
	defer sd.SecondDataExtended.Mutex.RUnlock()

	// 2. Get current period index
	periodIndex, err := getDayPeriodIndex()
	if err != nil {
		return false
	}

	// 3. Only run during regular market hours
	if periodIndex == 0 || periodIndex == 6 {
		return false // Skip pre/post market
	}

	// 4. Get opening range high/low (first 30 mins of trading)
	orHigh, orLow, err := getOpeningRange(sd)
	if err != nil {
		return false
	}

	// 5. Get current price from most recent bar
	if sd.SecondDataExtended.Size == 0 {
		return false
	}
	currentBar := sd.SecondDataExtended.Aggs[0]
	currentPrice := currentBar[3] // Close price

	// 6. Check if price has broken above ORH or below ORL
	if currentPrice > orHigh || currentPrice < orLow {
		return true
	}

	return false
}

func getOpeningRange(sd *socket.SecurityData) (float64, float64, error) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, 0, err
	}

	now := time.Now().In(location)
	marketOpen := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, location)
	orEnd := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, location)

	orHigh := -1.0
	orLow := 999999.0

	// Iterate through the bars to find OR high/low
	for i := 0; i < sd.SecondDataExtended.Size; i++ {
		bar := sd.SecondDataExtended.Aggs[i]
		barTime := now.Add(-time.Duration(i) * time.Second)

		// Only consider bars within the opening range period
		if barTime.Before(orEnd) && barTime.After(marketOpen) || barTime.Equal(marketOpen) {
			high := bar[1]
			low := bar[2]

			if high > orHigh {
				orHigh = high
			}
			if low < orLow {
				orLow = low
			}
		}
	}

	if orHigh == -1.0 || orLow == 999999.0 {
		return 0, 0, err
	}

	return orHigh, orLow, nil
}
