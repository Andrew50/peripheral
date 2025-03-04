package socket

import (
	"backend/utils"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

const (
	AggsLength = 100
	OHLCV      = 5
	Second     = 1
	Minute     = 60
	Hour       = 3600
	Day        = 86400

	concurrency = 30
)

const (
	secondAggs = true
	minuteAggs = false
	hourAggs   = false
	dayAggs    = false
	volBurst   = true
)

var (
	AggData      = make(map[int]*SecurityData)
	AggDataMutex sync.RWMutex

	// Add initialization tracking
	aggsInitialized     bool
	aggsInitializedLock sync.RWMutex
)

type TimeframeData struct {
	Aggs              [][]float64
	Size              int
	rolloverTimestamp int64
	extendedHours     bool
	Mutex             sync.RWMutex
}
type VolBurstData struct {
	VolumeThreshold []float64
	PriceThreshold  []float64
}

type SecurityData struct {
	SecondDataExtended TimeframeData
	MinuteDataExtended TimeframeData
	HourData           TimeframeData
	DayData            TimeframeData
	Dolvol             float64
	Mcap               float64
	Adr                float64
	VolBurstData       VolBurstData
}

func updateTimeframe(td *TimeframeData, timestamp int64, price float64, volume float64, timeframe int) {
	td.Mutex.Lock()         // Acquire write lock
	defer td.Mutex.Unlock() // Ensure the lock is released

	if timestamp >= td.rolloverTimestamp { // if out of order ticks
		if td.Size > 0 {
			copy(td.Aggs[1:], td.Aggs[0:min(td.Size, AggsLength-1)])
		}
		td.Aggs[0] = []float64{price, price, price, price, volume}
		td.rolloverTimestamp = nextPeriodStart(timestamp, timeframe)
		if td.Size < AggsLength {
			td.Size++
		}
	} else {
		td.Aggs[0][1] = max(td.Aggs[0][1], price)   // High
		td.Aggs[0][2] = min64(td.Aggs[0][2], price) // Low
		td.Aggs[0][3] = price                       // Close
		td.Aggs[0][4] += float64(volume)            // Volume
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
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

func appendTick(conn *utils.Conn, securityId int, timestamp int64, price float64, intVolume int64) error {
	// Check if aggs are initialized
	if !areAggsInitialized() {
		return fmt.Errorf("aggregates not yet initialized")
	}

	volume := float64(intVolume)

	AggDataMutex.RLock() // Acquire read lock
	sd, exists := AggData[securityId]
	AggDataMutex.RUnlock() // Release read lock

	if !exists {
		return fmt.Errorf("fid0w0f")
	}

	if utils.IsTimestampRegularHours(time.Unix(timestamp, timestamp*int64(time.Millisecond))) {
		if hourAggs {
			updateTimeframe(&sd.HourData, timestamp, price, volume, Hour)
		}
		if dayAggs {
			updateTimeframe(&sd.DayData, timestamp, price, volume, Day)
		}
	}

	if secondAggs {
		updateTimeframe(&sd.SecondDataExtended, timestamp, price, volume, Second)
	}

	if minuteAggs {
		updateTimeframe(&sd.MinuteDataExtended, timestamp, price, volume, Minute)
	}

	return nil
}

/*
	func getPeriodStart(timestamp int64, tf int) int64 {
	    return timestamp - (timestamp % int64(tf))
	}
*/
func nextPeriodStart(timestamp int64, tf int) int64 {
	return timestamp - (timestamp % int64(tf)) + int64(tf)
}

func GetTimeframeData(securityId int, timeframe int, extendedHours bool) ([][]float64, error) {
	AggDataMutex.RLock() // Acquire read lock
	sd, exists := AggData[securityId]
	AggDataMutex.RUnlock() // Release read lock

	if !exists {
		return nil, errors.New("security not found")
	}
	var td *TimeframeData
	switch timeframe {
	case Second:
		if extendedHours {
			td = &sd.SecondDataExtended
		}
	case Minute:
		if extendedHours {
			td = &sd.MinuteDataExtended
		}
	case Hour:
		td = &sd.HourData
	case Day:
		td = &sd.DayData
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
func InitAggregatesAsync(conn *utils.Conn) {
	setAggsInitialized(false)
	fmt.Println("Starting aggregates initialization in background...")
	go func() {
		if err := initAggregatesInternal(conn); err != nil {
			fmt.Printf("Error initializing aggregates: %v\n", err)
			return
		}
		setAggsInitialized(true)
		fmt.Println("✅ Aggregates initialization completed - Ready to process ticks")
	}()
}

// Internal function that does the actual initialization work
func initAggregatesInternal(conn *utils.Conn) error {
	fmt.Println("Loading historical data and initializing aggregates...")
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
	var securityIds []int
	for rows.Next() {
		var securityId int
		if err := rows.Scan(&securityId); err != nil {
			return fmt.Errorf("scanning security ID: %w", err)
		}
		securityIds = append(securityIds, securityId)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating security rows: %w", err)
	}

	// Process securities with a capped number of goroutines
	results := processSecuritiesConcurrently(securityIds, concurrency, conn)

	// Process results and collect errors
	data := make(map[int]*SecurityData)
	var loadErrors []error

	for _, res := range results {
		if res.err != nil {
			loadErrors = append(loadErrors, res.err)
		} else {
			data[res.securityId] = res.data
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
	securityId int
	data       *SecurityData
	err        error
}

func processSecuritiesConcurrently(securityIds []int, concurrency int, conn *utils.Conn) []result {
	resultsChan := make(chan result, len(securityIds))
	jobChan := make(chan int, len(securityIds))

	// Spawn a fixed number of workers
	for w := 0; w < concurrency; w++ {
		go func() {
			for sid := range jobChan {
				sd, err := processOneSecurity(sid, conn)
				resultsChan <- result{securityId: sid, data: sd, err: err}
			}
		}()
	}

	// Feed all security IDs into the job channel
	for _, sid := range securityIds {
		jobChan <- sid
	}
	close(jobChan)

	// Collect all results
	var results []result
	for i := 0; i < len(securityIds); i++ {
		r := <-resultsChan
		results = append(results, r)
	}
	close(resultsChan)

	return results
}

// Function to process a single security
func processOneSecurity(sid int, conn *utils.Conn) (*SecurityData, error) {
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
		fmt.Println("Security data is nil")
		return fmt.Errorf("security data is nil")
	}

	if secondAggs {
		if err := validateTimeframeData(&sd.SecondDataExtended, "second", true); err != nil {
			fmt.Printf("Second data validation failed: %v\n", err)
			return fmt.Errorf("second data validation failed: %w", err)
		}
	}

	if minuteAggs {
		if err := validateTimeframeData(&sd.MinuteDataExtended, "minute", true); err != nil {
			fmt.Printf("Minute data validation failed: %v\n", err)
			return fmt.Errorf("minute data validation failed: %w", err)
		}
	}

	if hourAggs {
		if err := validateTimeframeData(&sd.HourData, "hour", false); err != nil {
			fmt.Printf("Hour data validation failed: %v\n", err)
			return fmt.Errorf("hour data validation failed: %w", err)
		}
	}

	if dayAggs {
		if err := validateTimeframeData(&sd.DayData, "day", false); err != nil {
			fmt.Printf("Day data validation failed: %v\n", err)
			return fmt.Errorf("day data validation failed: %w", err)
		}
	}

	return nil
}

// Helper function to validate timeframe data
func validateTimeframeData(td *TimeframeData, timeframeName string, extendedHours bool) error {
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

func initTimeframeData(conn *utils.Conn, securityId int, timeframe int, isExtendedHours bool) TimeframeData {
	aggs := make([][]float64, AggsLength)
	for i := range aggs {
		aggs[i] = make([]float64, OHLCV)
	}
	td := TimeframeData{
		Aggs:              aggs,
		Size:              0,
		rolloverTimestamp: -1,
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
		fmt.Printf("Invalid timeframe: %d\n", timeframe)
		return td
	}
	fromMillis, toMillis, err := utils.GetRequestStartEndTime(time.Unix(0, 0), toTime, "backward", tfStr, multiplier, AggsLength)
	if err != nil {
		fmt.Printf("error g2io002")
		return td
	}
	ticker, err := utils.GetTicker(conn, securityId, toTime)
	//ticker := obj.ticekr
	if err != nil {
		fmt.Printf("error getting hist data")
		return td
	}
	iter, err := utils.GetAggsData(conn.Polygon, ticker, multiplier, tfStr, fromMillis, toMillis, AggsLength, "desc", true)
	if err != nil {
		fmt.Printf("Error getting historical data: %v\n", err)
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
	if err := iter.Err(); err != nil {
		fmt.Printf("Error iterating historical data: %v\n", err)
	}
	//if td.rolloverTimestamp == -1 {
	td.rolloverTimestamp = lastTimestamp + int64(timeframe) //if theres no data then this wont work but is extreme edge case
	//}
	td.Size = idx
	return td
}

// initVolBurstData initializes threshold data for volume burst (tape burst)
// detection based on historical trading data. We define seven periods:
//
//	0: premarket (4:00 - 9:30)
//	1: open 9:30 - 9:45
//	2: 9:45 - 10:00
//	3: 10:00 - 12:00
//	4: 12:00 - 14:00
//	5: 14:00 - 16:00
//	6: after hours (16:00 - 20:00)
//
// For each period, we break the interval into 20 second segments and compute
// the average total volume and average % price range (from high to low) over those windows.
// The resulting thresholds are saved in the order specified.
func initVolBurstData(conn *utils.Conn, securityId int) VolBurstData {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		fmt.Printf("error getting location: %v\n", err)
		return VolBurstData{}
	}
	now := time.Now().In(location)
	// Single fetch for the last 15 days
	globalStart := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, location).AddDate(0, 0, -15)
	globalEnd := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, location)

	ticker, err := utils.GetTicker(conn, securityId, now)
	if err != nil {
		fmt.Printf("error getting ticker: %v\n", err)
		return VolBurstData{}
	}

	fromMillis := models.Millis(globalStart)
	endMillis := models.Millis(globalEnd)

	iter, err := utils.GetAggsData(conn.Polygon, ticker, 1, "second", fromMillis, endMillis, 100000, "asc", true)
	if err != nil {
		fmt.Printf("error getting aggs data: %v\n", err)
		return VolBurstData{}
	}

	// Read all second bars
	var allAggs []models.Agg
	for iter.Next() {
		allAggs = append(allAggs, iter.Item())
	}
	// Separate them by day
	dailyBars := make(map[time.Time][]models.Agg)
	for _, agg := range allAggs {
		t := time.Time(agg.Timestamp).In(location)
		dateKey := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, location)
		dailyBars[dateKey] = append(dailyBars[dateKey], agg)
	}

	// Predefine the 7 time-of-day "periods"
	// We'll reuse this for each day
	type Period struct {
		Start, End time.Time
	}

	// We'll accumulate totalVol & totalPct in 7 buckets across all days
	totalVol := make([]float64, 7)
	totalPct := make([]float64, 7)
	countWin := make([]int, 7)

	// For each of the last 15 days
	for d := 0; d < 15; d++ {
		dayKey := now.AddDate(0, 0, -d)
		midnight := time.Date(dayKey.Year(), dayKey.Month(), dayKey.Day(), 0, 0, 0, 0, location)

		// Actual day boundaries
		dayStart := midnight.Add(4 * time.Hour) // 4:00
		dayEnd := midnight.Add(20 * time.Hour)  // 20:00
		bars := dailyBars[midnight]             // might be nil if no data that day
		if len(bars) == 0 {
			continue
		}

		// We'll do a 2-step approach:
		//   1) We'll define the 7 time-based sub-periods for *that day*.
		//   2) We'll compute the total volume / pct range stats for each sub-period via our sliding window.

		subPeriods := []struct {
			idx       int
			startTime time.Time
			endTime   time.Time
		}{
			{0, dayStart, time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 9, 30, 0, 0, location)}, // pre
			{1, time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 9, 30, 0, 0, location),
				time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 9, 45, 0, 0, location)},
			{2, time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 9, 45, 0, 0, location),
				time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 10, 0, 0, 0, location)},
			{3, time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 10, 0, 0, 0, location),
				time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 12, 0, 0, 0, location)},
			{4, time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 12, 0, 0, 0, location),
				time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 14, 0, 0, 0, location)},
			{5, time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 14, 0, 0, 0, location),
				time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 16, 0, 0, 0, location)},
			{6, time.Date(midnight.Year(), midnight.Month(), midnight.Day(), 16, 0, 0, 0, location), dayEnd},
		}

		// For each sub-period, run the sliding window logic
		for _, sp := range subPeriods {
			if sp.endTime.Before(sp.startTime) {
				continue
			}
			if sp.endTime.After(dayEnd) {
				sp.endTime = dayEnd
			}

			vol, pct, cnt := slidingWindowStats(bars, sp.startTime, sp.endTime)
			totalVol[sp.idx] += vol
			totalPct[sp.idx] += pct
			countWin[sp.idx] += cnt
		}
	}

	// Compute the final averages
	volThreshold := make([]float64, 7)
	priceThreshold := make([]float64, 7)
	for i := 0; i < 7; i++ {
		if countWin[i] > 0 {
			volThreshold[i] = totalVol[i] / float64(countWin[i])
			priceThreshold[i] = totalPct[i] / float64(countWin[i])
		}
	}

	// Possibly debug print if it's a well-known ticker
	if ticker == "NVDA" || ticker == "TSLA" {
		fmt.Printf("volThreshold: %v, priceThreshold: %v, ticker %v\n", volThreshold, priceThreshold, ticker)
	}
	return VolBurstData{
		VolumeThreshold: volThreshold,
		PriceThreshold:  priceThreshold,
	}
}
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
}
func initSecurityData(conn *utils.Conn, securityId int) *SecurityData {
	sd := &SecurityData{}

	if secondAggs {
		sd.SecondDataExtended = initTimeframeData(conn, securityId, Second, true)
	}

	if minuteAggs {
		sd.MinuteDataExtended = initTimeframeData(conn, securityId, Minute, true)
	}

	if hourAggs {
		sd.HourData = initTimeframeData(conn, securityId, Hour, false)
	}

	if dayAggs {
		sd.DayData = initTimeframeData(conn, securityId, Day, false)
	}
	if volBurst {
		sd.VolBurstData = initVolBurstData(conn, securityId)
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
