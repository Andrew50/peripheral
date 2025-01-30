package alerts

import (
	"backend/utils"
	"context"
	"fmt"
	"sync"
	"time"
)

const concurrency = 30 // Set the desired level of concurrency
//const polygonRateLimit = 1000 //per second
// Define constants to control which aggregates are active

type result struct {
	securityId int
	data       *SecurityData
	err        error
}

var (
	aggsInitialized     bool
	aggsInitializedLock sync.RWMutex // Mutex to protect aggsInitialized
)

func InitAlertsAndAggs(conn *utils.Conn) error {
	setAggsInitialized(false) // Set to false at the start
	ctx := context.Background()

	if err := loadActiveAlerts(ctx, conn); err != nil {
		return fmt.Errorf("loading active alerts: %w", err)
	}

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

	// Validate alert securities exist in data map
	var alertErrors []error
	alerts.Range(func(key, value interface{}) bool {
		alert := value.(Alert)
		if alert.SecurityId != nil {
			if _, exists := data[*alert.SecurityId]; !exists {
				alertErrors = append(alertErrors,
					fmt.Errorf("alert ID %d references non-existent security ID %d",
						alert.AlertId, *alert.SecurityId))
			}
		}
		return true
	})

	// Report any alert validation errors
	if len(alertErrors) > 0 {
		var errMsg string
		for i, err := range alertErrors {
			if i > 0 {
				errMsg += "; "
			}
			errMsg += err.Error()
		}
		return fmt.Errorf("errors validating alerts: %s", errMsg)
	}

	alertAggDataMutex.Lock() // Acquire write lock
	alertAggData = data
	alertAggDataMutex.Unlock() // Release write lock

	setAggsInitialized(true) // Set to true after successful initialization
	fmt.Println("Finished initializing alerts and aggregates")
	return nil
}

// Function to safely set the aggsInitialized flag
func setAggsInitialized(value bool) {
	aggsInitializedLock.Lock()
	defer aggsInitializedLock.Unlock()
	aggsInitialized = value
	fmt.Println("debug: aggsInitialized:----------------------", aggsInitialized)
}

// Function to safely get the aggsInitialized flag
func IsAggsInitialized() bool {
	aggsInitializedLock.RLock()
	defer aggsInitializedLock.RUnlock()
	return aggsInitialized
}

// Function to process securities concurrently with a fixed number of workers
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

// Helper function to load active alerts
func loadActiveAlerts(ctx context.Context, conn *utils.Conn) error {
	query := `
        SELECT alertId, userId, alertType, setupId, price, direction, securityId
        FROM alerts
        WHERE active = true
    `
	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("querying active alerts: %w", err)
	}
	defer rows.Close()

	alerts = sync.Map{}
	for rows.Next() {
		var alert Alert
		err := rows.Scan(
			&alert.AlertId,
			&alert.UserId,
			&alert.AlertType,
			&alert.SetupId,
			&alert.Price,
			&alert.Direction,
			&alert.SecurityId,
		)
		if err != nil {
			return fmt.Errorf("scanning alert row: %w", err)
		}
		alerts.Store(alert.AlertId, alert)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating alert rows: %w", err)
	}

	return nil
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

	if len(td.Aggs) != Length {
		return fmt.Errorf("%s aggregates length mismatch: got %d, want %d",
			timeframeName, len(td.Aggs), Length)
	}

	if td.rolloverTimestamp == -1 {
		return fmt.Errorf("%s rollover timestamp not initialized", timeframeName)
	}

	return nil
}

func initTimeframeData(conn *utils.Conn, securityId int, timeframe int, isExtendedHours bool) TimeframeData {
	aggs := make([][]float64, Length)
	for i := range aggs {
		aggs[i] = make([]float64, OHLCV)
	}
	td := TimeframeData{
		Aggs:              aggs,
		size:              0,
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
	fromMillis, toMillis, err := utils.GetRequestStartEndTime(time.Unix(0, 0), toTime, "backward", tfStr, multiplier, Length)
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
	iter, err := utils.GetAggsData(conn.Polygon, ticker, multiplier, tfStr, fromMillis, toMillis, Length, "desc", true)
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

		if idx >= Length {

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
	td.size = idx
	return td
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

	return sd
}
