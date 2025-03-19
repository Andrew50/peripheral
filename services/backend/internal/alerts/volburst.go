package alerts

import (
	"backend/socket"
	"fmt"
	"time"
)

func isTapeBurst(sd *socket.SecurityData) bool {
	// 1. Lock second data
	sd.SecondDataExtended.Mutex.RLock()
	defer sd.SecondDataExtended.Mutex.RUnlock()
	// 2. If we don’t have enough bars, return false
	lookbackSeconds := 15
	if lookbackSeconds > sd.SecondDataExtended.Size {
		return false
	}
	periodIndex, err := getDayPeriodIndex()
	if err != nil {
		return false
	}
	if sd.VolBurstData.VolumeThreshold[periodIndex] < 300 {
		return false // minimum liquidity requirement
	}
	totalVol := 0.0
	windowHigh := sd.SecondDataExtended.Aggs[0][1]
	windowLow := sd.SecondDataExtended.Aggs[0][2]
	for i := 0; i < lookbackSeconds; i++ {
		bar := sd.SecondDataExtended.Aggs[i]
		vol := bar[4]
		high := bar[1]
		low := bar[2]
		totalVol += vol
		if high > windowHigh {
			windowHigh = high
		}
		if low < windowLow {
			windowLow = low
		}
	}
	if windowLow <= 0 {
		return false
	}
	pctRange := (windowHigh - windowLow) / windowLow

	// 5. Compare to thresholds from VolBurstData
	volThreshold := sd.VolBurstData.VolumeThreshold[periodIndex] * 10.0
	pctThreshold := sd.VolBurstData.PriceThreshold[periodIndex] * 10.0

	if totalVol >= volThreshold && pctRange >= pctThreshold {
		return true
	}
	return false
}

func getDayPeriodIndex() (int, error) {

	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, err
	}
	now := time.Now().In(location)
	preMarketStart := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, location)
	open := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, location)
	openEarlyEnd := time.Date(now.Year(), now.Month(), now.Day(), 9, 45, 0, 0, location)
	openLateEnd := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, location)
	h10to12 := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, location)
	h12to2 := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, location)
	close := time.Date(now.Year(), now.Month(), now.Day(), 16, 0, 0, 0, location)
	afterHoursEnd := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, location)
	switch {
	case now.Before(preMarketStart) || now.After(afterHoursEnd):
		return 0, fmt.Errorf("market closed") // or handle “extended extended hours” differently
	case now.After(preMarketStart) && now.Before(open):
		return 0, nil // premarket
	case now.Equal(open) || (now.After(open) && now.Before(openEarlyEnd)):
		return 1, nil
	case now.Equal(openEarlyEnd) || (now.After(openEarlyEnd) && now.Before(openLateEnd)):
		return 2, nil
	case now.Equal(openLateEnd) || (now.After(openLateEnd) && now.Before(h10to12)):
		return 3, nil
	case now.After(h10to12) && now.Before(h12to2):
		return 4, nil
	case now.After(h12to2) && now.Before(close):
		return 5, nil
	case now.After(close) && now.Before(afterHoursEnd):
		return 6, nil // after hours
	}
	return 0, fmt.Errorf("could not determine day period")
}
