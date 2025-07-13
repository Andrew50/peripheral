package socket

/*

import (
	"backend/internal/app/chart"
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/postgres"
	"backend/internal/data/utils"
	"context"
	"errors"
	"fmt"
	"time"
)

/*
import (

	"backend/internal/app/chart"
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/postgres"
	"backend/internal/data/utils"

	"context"
	"errors"
	"fmt"
	"sync"
	"time"

)

const (

	AggsLength = 200
	OHLCV      = 5
	Second     = 1
	Minute     = 60
	Hour       = 3600
	Day        = 86400

	concurrency = 10

	secondAggs = true
	minuteAggs = false
	hourAggs   = false
	dayAggs    = false

)

var (

	AggData      = make(map[int]*SecurityData)
	AggDataMutex sync.RWMutex

	// Add initialization tracking
	aggsInitialized     bool
	aggsInitializedLock sync.RWMutex

)

// TimeframeData represents a structure for handling TimeframeData data.

	type TimeframeData struct {
		Aggs              [][]float64
		Size              int
		rolloverTimestamp int64
		extendedHours     bool
		Mutex             sync.RWMutex
	}

// SecurityData represents a structure for handling SecurityData data.

	type SecurityData struct {
		SecondDataExtended *TimeframeData
		MinuteDataExtended *TimeframeData
		HourData           *TimeframeData
		DayData            *TimeframeData
		Dolvol             float64
		Mcap               float64
		Adr                float64
	}

	func updateTimeframe(td *TimeframeData, timestamp int64, price float64, volume float64, timeframe int) {
		td.Mutex.Lock()         // Acquire write lock
		defer td.Mutex.Unlock() // Ensure the lock is released

		if timestamp >= td.rolloverTimestamp { // if out of order ticks
			if td.Size > 0 {
				copy(td.Aggs[1:], td.Aggs[0:minIntsAggs(td.Size, AggsLength-1)])
			}
			td.Aggs[0] = []float64{price, price, price, price, volume}
			td.rolloverTimestamp = nextPeriodStart(timestamp, timeframe)
			if td.Size < AggsLength {
				td.Size++
			}
		} else {
			td.Aggs[0][1] = maxFloat64Aggs(td.Aggs[0][1], price) // High
			td.Aggs[0][2] = min64(td.Aggs[0][2], price)          // Low
			td.Aggs[0][3] = price                                // Close
			td.Aggs[0][4] += float64(volume)                     // Volume
		}
	}

	// updateTimeframeVolumeOnly updates only volume without affecting price data
	// Used when price is -1 (skip OHLC condition) but volume should still be updated
	func updateTimeframeVolumeOnly(td *TimeframeData, timestamp int64, volume float64, timeframe int) {
		td.Mutex.Lock()         // Acquire write lock
		defer td.Mutex.Unlock() // Ensure the lock is released

		if timestamp >= td.rolloverTimestamp { // if out of order ticks
			if td.Size > 0 {
				copy(td.Aggs[1:], td.Aggs[0:minIntsAggs(td.Size, AggsLength-1)])
			}
			// Use the last valid price from the previous bar, or 0 if no previous data
			var lastPrice float64
			if td.Size > 0 {
				lastPrice = td.Aggs[0][3] // Use last close price
			}
			td.Aggs[0] = []float64{lastPrice, lastPrice, lastPrice, lastPrice, volume}
			td.rolloverTimestamp = nextPeriodStart(timestamp, timeframe)
			if td.Size < AggsLength {
				td.Size++
			}
		} else {
			// Only update volume, leave price data unchanged
			td.Aggs[0][4] += float64(volume) // Volume
		}
	}

	func minIntsAggs(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	func maxFloat64Aggs(a, b float64) float64 {
		if a > b {
			return a
		}
		return b
	}

	func min64(a, b float64) float64 {
		if a < b {
			return a
		}
		return b
	}

// Function to check if aggs are initialized

	func areAggsInitialized() bool {
		aggsInitializedLock.RLock()
		defer aggsInitializedLock.RUnlock()
		return aggsInitialized
	}

// Function to set aggs initialization status

	func setAggsInitialized(value bool) {
		aggsInitializedLock.Lock()
		defer aggsInitializedLock.Unlock()
		aggsInitialized = value
	}

	func appendTick(_ *data.Conn, securityID int, timestamp int64, price float64, intVolume int64) error {
		// Check if aggs are initialized
		if !areAggsInitialized() {
			return fmt.Errorf("aggregates not yet initialized")
		}

		volume := float64(intVolume)

		// Skip price updates if price is -1 (indicates skip OHLC condition)
		// but still allow volume updates
		shouldSkipPriceUpdate := price < 0

		AggDataMutex.RLock() // Acquire read lock
		sd, exists := AggData[securityID]
		AggDataMutex.RUnlock() // Release read lock

		if !exists {
			return fmt.Errorf("fid0w0f")
		}

		// Only update price-related aggregations if not skipping price updates
		if !shouldSkipPriceUpdate {
			if utils.IsTimestampRegularHours(time.Unix(timestamp, timestamp*int64(time.Millisecond))) {
				if hourAggs {
					updateTimeframe(sd.HourData, timestamp, price, volume, Hour)
				}
				if dayAggs {
					updateTimeframe(sd.DayData, timestamp, price, volume, Day)
				}
			}

			if secondAggs {
				updateTimeframe(sd.SecondDataExtended, timestamp, price, volume, Second)
			}

			if minuteAggs {
				updateTimeframe(sd.MinuteDataExtended, timestamp, price, volume, Minute)
			}
		} else {
			// When skipping price updates, only update volume if volume > 0
			if volume > 0 {
				if utils.IsTimestampRegularHours(time.Unix(timestamp, timestamp*int64(time.Millisecond))) {
					if hourAggs {
						updateTimeframeVolumeOnly(sd.HourData, timestamp, volume, Hour)
					}
					if dayAggs {
						updateTimeframeVolumeOnly(sd.DayData, timestamp, volume, Day)
					}
				}

				if secondAggs {
					updateTimeframeVolumeOnly(sd.SecondDataExtended, timestamp, volume, Second)
				}

				if minuteAggs {
					updateTimeframeVolumeOnly(sd.MinuteDataExtended, timestamp, volume, Minute)
				}
			}
		}

		return nil
	}

/*

	func getPeriodStart(timestamp int64, tf int) int64 {
	    return timestamp - (timestamp % int64(tf))
	}
*/
/*
func nextPeriodStart(timestamp int64, tf int) int64 {
	return timestamp - (timestamp % int64(tf)) + int64(tf)
}

// GetTimeframeData performs operations related to GetTimeframeData functionality.
func GetTimeframeData(securityID int, timeframe int, extendedHours bool) ([][]float64, error) {
	AggDataMutex.RLock() // Acquire read lock
	sd, exists := AggData[securityID]
	AggDataMutex.RUnlock() // Release read lock

	if !exists {
		return nil, errors.New("security not found")
	}
	var td *TimeframeData
	switch timeframe {
	case Second:
		if extendedHours {
			td = sd.SecondDataExtended
		}
	case Minute:
		if extendedHours {
			td = sd.MinuteDataExtended
		}
	case Hour:
		td = sd.HourData
	case Day:
		td = sd.DayData
	default:
		return nil, errors.New("invalid timeframe")
	}
	if td == nil {
		return nil, errors.New("timeframe data not available")
	}
	td.Mutex.RLock()
	defer td.Mutex.RUnlock()
	result := make([][]float64, len(td.Aggs))
	for i := range td.Aggs {
		result[i] = make([]float64, OHLCV)
		copy(result[i], td.Aggs[i])
	}
	return result, nil
}

// InitAggregatesAsync starts the initialization process in a goroutine
func InitAggregatesAsync(conn *data.Conn) {
	setAggsInitialized(false)
	////fmt.Println("Starting aggregates initialization in background...")
	go func() {
		if err := initAggregatesInternal(conn); err != nil {
			////fmt.Printf("Error initializing aggregates: %v\n", err)
			return
		}
		setAggsInitialized(true)
		////fmt.Println("✅ Aggregates initialization completed - Ready to process ticks")
	}()
}

// Internal function that does the actual initialization work
func initAggregatesInternal(conn *data.Conn) error {
	////fmt.Println("Loading historical data and initializing aggregates...")
	ctx := context.Background()

	query := `
        SELECT securityId
        FROM securities
        WHERE maxDate is NULL = true`

	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("querying securities: %w", err)
	}

	defer rows.Close()
	var securityIDs []int
	for rows.Next() {
		var securityID int
		if err := rows.Scan(&securityID); err != nil {
			return fmt.Errorf("scanning security ID: %w", err)
		}
		securityIDs = append(securityIDs, securityID)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating security rows: %w", err)
	}

	// Process securities with a capped number of goroutines
	results := processSecuritiesConcurrently(securityIDs, concurrency, conn)

	// Process results and collect errors
	data := make(map[int]*SecurityData)
	var loadErrors []error

	for _, res := range results {
		if res.err != nil {
			loadErrors = append(loadErrors, res.err)
		} else {
			data[res.securityID] = res.data
		}
	}

	// Handle any load errors
	if len(loadErrors) > 0 {
		var errMsg string
		for i, err := range loadErrors {
			if i > 0 {
				errMsg += "; "
			}
			errMsg += err.Error()
		}
		return fmt.Errorf("errors loading aggregates: %s", errMsg)
	}

	AggDataMutex.Lock() // Acquire write lock
	AggData = data
	AggDataMutex.Unlock() // Release write lock

	return nil
}

// Function to process securities concurrently with a fixed number of workers
type result struct {
	securityID int
	data       *SecurityData
	err        error
}

func processSecuritiesConcurrently(securityIDs []int, concurrency int, conn *data.Conn) []result {
	resultsChan := make(chan result, len(securityIDs))
	jobChan := make(chan int, len(securityIDs))

	// Spawn a fixed number of workers
	for w := 0; w < concurrency; w++ {
		go func() {
			for sid := range jobChan {
				sd, err := processOneSecurity(sid, conn)
				resultsChan <- result{securityID: sid, data: sd, err: err}
			}
		}()
	}

	// Feed all security IDs into the job channel
	for _, sid := range securityIDs {
		jobChan <- sid
	}
	close(jobChan)

	// Collect all results
	var results []result
	for i := 0; i < len(securityIDs); i++ {
		r := <-resultsChan
		results = append(results, r)
	}
	close(resultsChan)

	return results
}

// Function to process a single security
func processOneSecurity(sid int, conn *data.Conn) (*SecurityData, error) {
	sd := initSecurityData(conn, sid)
	if sd == nil {
		return nil, fmt.Errorf("failed to initialize security data for ID %d", sid)
	}

	if err := validateSecurityData(sd); err != nil {
		return nil, fmt.Errorf("validation failed for security %d: %w", sid, err)
	}

	return sd, nil
}

// Helper function to validate initialized security data
func validateSecurityData(sd *SecurityData) error {
	if sd == nil {
		////fmt.Println("Security data is nil")
		return fmt.Errorf("security data is nil")
	}

	if secondAggs {
		if err := validateTimeframeData(sd.SecondDataExtended, "second", true); err != nil {
			////fmt.Printf("Second data validation failed: %v\n", err)
			return fmt.Errorf("second data validation failed: %w", err)
		}
	}

	if minuteAggs {
		if err := validateTimeframeData(sd.MinuteDataExtended, "minute", true); err != nil {
			////fmt.Printf("Minute data validation failed: %v\n", err)
			return fmt.Errorf("minute data validation failed: %w", err)
		}
	}

	if hourAggs {
		if err := validateTimeframeData(sd.HourData, "hour", false); err != nil {
			////fmt.Printf("Hour data validation failed: %v\n", err)
			return fmt.Errorf("hour data validation failed: %w", err)
		}
	}

	if dayAggs {
		if err := validateTimeframeData(sd.DayData, "day", false); err != nil {
			////fmt.Printf("Day data validation failed: %v\n", err)
			return fmt.Errorf("day data validation failed: %w", err)
		}
	}

	return nil
}

// Helper function to validate timeframe data
func validateTimeframeData(td *TimeframeData, timeframeName string, _ bool) error {
	if td == nil {
		return fmt.Errorf("%s timeframe data is nil", timeframeName)
	}

	if td.Aggs == nil {
		return fmt.Errorf("%s aggregates array is nil", timeframeName)
	}

	if len(td.Aggs) != AggsLength {
		return fmt.Errorf("%s aggregates length mismatch: got %d, want %d",
			timeframeName, len(td.Aggs), AggsLength)
	}

	if td.rolloverTimestamp == -1 {
		return fmt.Errorf("%s rollover timestamp not initialized", timeframeName)
	}

	return nil
}

func initTimeframeData(conn *data.Conn, securityID int, timeframe int, isExtendedHours bool) *TimeframeData {
	aggs := make([][]float64, AggsLength)
	for i := range aggs {
		aggs[i] = make([]float64, OHLCV)
	}
	td := &TimeframeData{ // Initialize as a pointer
		Aggs:              aggs,
		Size:              0,
		rolloverTimestamp: -1, // Mark as not properly initialized until data is loaded
		extendedHours:     isExtendedHours,
	}
	toTime := time.Now()
	var tfStr string
	var multiplier int
	switch timeframe {
	case Second:
		tfStr = "second"
		multiplier = 1
	case Minute:
		tfStr = "minute"
		multiplier = 1
	case Hour:
		tfStr = "hour"
		multiplier = 1
	case Day:
		tfStr = "day"
		multiplier = 1
	default:
		////fmt.Printf("Invalid timeframe: %d\n", timeframe)
		return td
	}
	fromMillis, toMillis, err := chart.GetRequestStartEndTime(time.Unix(0, 0), toTime, "backward", tfStr, multiplier, AggsLength)
	if err != nil {
		////fmt.Printf("error g2io002")
		return td
	}
	ticker, err := postgres.GetTicker(conn, securityID, toTime)
	//ticker := obj.ticekr
	if err != nil {
		////fmt.Printf("error getting hist data")
		return td
	}
	iter, err := polygon.GetAggsData(conn.Polygon, ticker, multiplier, tfStr, fromMillis, toMillis, AggsLength, "desc", true)
	if err != nil {
		////fmt.Printf("Error getting historical data: %v\n", err)
		return td
	}

	// Process historical data
	var idx int
	var lastTimestamp int64
	for iter.Next() {
		agg := iter.Item()

		// Skip if we're not including extended hours data
		timestamp := time.Time(agg.Timestamp)
		if !isExtendedHours && !utils.IsTimestampRegularHours(timestamp) {
			continue
		}

		if idx >= AggsLength {

			break
		}

		td.Aggs[idx] = []float64{
			agg.Open,
			agg.High,
			agg.Low,
			agg.Close,
			float64(agg.Volume),
		}

		idx++
		lastTimestamp = time.Time(agg.Timestamp).Unix()
	}
	//	if err := iter.Err(); err != nil {
	////fmt.Printf("Error iterating historical data: %v\n", err)
	//}
	//if td.rolloverTimestamp == -1 {
	td.rolloverTimestamp = lastTimestamp + int64(timeframe) //if theres no data then this wont work but is extreme edge case
	//}
	td.Size = idx
	return td
} /*
func slidingWindowStats(bars []models.Agg, startTime, endTime time.Time) (float64, float64, int) {
	const windowDur = 20 * time.Second
	const slideStep = 5 * time.Second
	// Ensure bars are sorted ascending by timestamp
	// Typically they'd already be sorted from your polygon call.

	totalVol := 0.0
	totalPct := 0.0
	count := 0

	leftIdx := 0
	for t := startTime; t.Before(endTime); t = t.Add(slideStep) {
		windowStart := t
		windowEnd := t.Add(windowDur)
		if windowEnd.After(endTime) {
			windowEnd = endTime
		}

		// Move leftIdx to skip bars older than windowStart
		for leftIdx < len(bars) && time.Time(bars[leftIdx].Timestamp).Before(windowStart) {
			leftIdx++
		}
		if leftIdx >= len(bars) {
			break
		}
		// Accumulate stats for bars in [windowStart, windowEnd]
		var (
			wVol float64
			wMin float64
			wMax float64
			init bool
		)
		for rightIdx := leftIdx; rightIdx < len(bars); rightIdx++ {
			ts := time.Time(bars[rightIdx].Timestamp)
			if ts.After(windowEnd) {
				break
			}
			wVol += bars[rightIdx].Volume
			if !init {
				wMin = bars[rightIdx].Low
				wMax = bars[rightIdx].High
				init = true
			} else {
				if bars[rightIdx].Low < wMin {
					wMin = bars[rightIdx].Low
				}
				if bars[rightIdx].High > wMax {
					wMax = bars[rightIdx].High
				}
			}
		}
		if init && wMin > 0 {
			pct := (wMax - wMin) / wMin
			totalVol += wVol
			totalPct += pct
			count++
		}
	}

	return totalVol, totalPct, count
}*/
/*
func initSecurityData(conn *data.Conn, securityID int) *SecurityData {
	sd := &SecurityData{}

	if secondAggs {
		sd.SecondDataExtended = initTimeframeData(conn, securityID, Second, true)
	}

	if minuteAggs {
		sd.MinuteDataExtended = initTimeframeData(conn, securityID, Minute, true)
	}

	if hourAggs {
		sd.HourData = initTimeframeData(conn, securityID, Hour, false)
	}

	if dayAggs {
		sd.DayData = initTimeframeData(conn, securityID, Day, false)
	}
	return sd
}

/*
func appendAggregate(securityId int,timeframe string, o float64, h float64, l float64, c float64) error {
    sd, exists := data[securityId]
    if !exists {
        sd = initSecurityData()
        data[securityId] = sd
    }
    sd.mutex.Lock()
    defer sd.mutex.Unlock()

    if sd.size > 0 {
        copy(sd.Aggs[1:],sd.Aggs[0:min(sd.size,Length-1)])
    }
    sd.Aggs[0] = []float64{o,h,l,c}
    if sd.size < Length {
        sd.size ++
    }
    return nil
}*/
