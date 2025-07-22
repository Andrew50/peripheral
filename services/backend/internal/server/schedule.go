package server

import (
	"backend/internal/data"
	"backend/internal/services/alerts"
	"backend/internal/services/marketdata"
	"backend/internal/services/screener"
	"backend/internal/services/securities"
	"backend/internal/services/socket"
	"backend/internal/services/subscriptions"
	"backend/internal/services/worker_monitor"
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"
	//"github.com/go-redis/redis/v8"
)

var useBS = false //alerts, securityUpdate, marketMetrics, sectorUpdate

var (
	polygonInitialized bool
	polygonInitMutex   sync.Mutex
	alertsInitialized  bool
	alertsInitMutex    sync.Mutex
	workerMonitor      *worker_monitor.WorkerMonitor
	workerMonitorMutex sync.Mutex
)

// Global flags to track services started by partial coverage check
var (
	screenerStartedByPartialCoverage bool
	polygonStartedByPartialCoverage  bool
	partialCoverageCheckMutex        sync.Mutex
)

// JobFunc represents a function that can be executed as a job
type JobFunc func(conn *data.Conn) error

// TimeOfDay represents a specific time during the day (hour and minute)
type TimeOfDay struct {
	Hour   int
	Minute int
}

// Job represents a scheduled job
type Job struct {
	Name               string
	Function           JobFunc
	Schedule           []TimeOfDay
	LastRun            time.Time // This will be loaded from Redis but kept in memory for quick access
	LastCompletionTime time.Time // Tracks when the job was verified to have completed successfully
	RunOnInit          bool
	ExecutionMutex     sync.Mutex
	IsRunning          bool
	SkipOnWeekends     bool
	RetryOnFailure     bool          // Whether to retry the job on failure
	MaxRetries         int           // Maximum number of retry attempts
	RetryDelay         time.Duration // Delay between retry attempts
}

// JobScheduler manages and executes jobs
type JobScheduler struct {
	Jobs      []*Job
	Conn      *data.Conn
	Location  *time.Location
	StopChan  chan struct{}
	IsRunning bool
	mutex     sync.Mutex
}

// Redis key prefix for job last run times
const jobLastRunKeyPrefix = "job:lastrun:"
const jobLastCompletionKeyPrefix = "job:lastcompletion:"
const jobRetryCountKeyPrefix = "job:retrycount:"

// getJobLastRunKey returns the Redis key for storing a job's last run time
func getJobLastRunKey(jobName string) string {
	return jobLastRunKeyPrefix + jobName
}

// getJobLastCompletionKey returns the Redis key for storing a job's last completion time
func getJobLastCompletionKey(jobName string) string {
	return jobLastCompletionKeyPrefix + jobName
}

// getJobRetryCountKey returns the Redis key for storing a job's retry count
func getJobRetryCountKey(jobName string) string {
	return jobRetryCountKeyPrefix + jobName
}

// loadJobLastRunTimes loads the last run times for all jobs from Redis
func (s *JobScheduler) loadJobLastRunTimes() {
	ctx := context.Background()

	for _, job := range s.Jobs {
		// Get the last run time from Redis
		lastRunStr, err := s.Conn.Cache.Get(ctx, getJobLastRunKey(job.Name)).Result()
		if err == nil && lastRunStr != "" {
			// Parse the timestamp
			lastRun, err := time.Parse(time.RFC3339, lastRunStr)
			if err == nil {
				job.LastRun = lastRun
			}
		}
		//else if err != redis.Nil {
		// Error loading last run time, other than not found
		//}

		// Get the last completion time from Redis
		lastCompletionStr, err := s.Conn.Cache.Get(ctx, getJobLastCompletionKey(job.Name)).Result()
		if err == nil && lastCompletionStr != "" {
			// Parse the timestamp
			lastCompletion, err := time.Parse(time.RFC3339, lastCompletionStr)
			if err == nil {
				job.LastCompletionTime = lastCompletion
			}
		}
		//else if err != redis.Nil {
		// Error loading last completion time, other than not found
		//}
	}
}

// saveJobLastRunTime saves a job's last run time to Redis
func (s *JobScheduler) saveJobLastRunTime(job *Job) error {
	ctx := context.Background()

	// Store the last run time in Redis
	lastRunStr := job.LastRun.Format(time.RFC3339)
	err := s.Conn.Cache.Set(ctx, getJobLastRunKey(job.Name), lastRunStr, 0).Err()
	return err
	//if err != nil {

	// Log error saving last run time
	//}
}

// saveJobLastCompletionTime saves a job's last completion time to Redis
func (s *JobScheduler) saveJobLastCompletionTime(job *Job) error {
	ctx := context.Background()

	// Store the last completion time in Redis
	lastCompletionStr := job.LastCompletionTime.Format(time.RFC3339)
	err := s.Conn.Cache.Set(ctx, getJobLastCompletionKey(job.Name), lastCompletionStr, 0).Err()
	return err
}

// saveJobRetryCount saves a job's retry count to Redis
func (s *JobScheduler) saveJobRetryCount(job *Job, retryCount int) error {
	ctx := context.Background()
	err := s.Conn.Cache.Set(ctx, getJobRetryCountKey(job.Name), retryCount, 0).Err()
	return err
}

// loadJobRetryCount loads a job's retry count from Redis
func (s *JobScheduler) loadJobRetryCount(job *Job) int {
	ctx := context.Background()
	retryCountStr, err := s.Conn.Cache.Get(ctx, getJobRetryCountKey(job.Name)).Result()
	if err == nil && retryCountStr != "" {
		if count, err := strconv.Atoi(retryCountStr); err == nil {
			return count
		}
	}
	return 0
}

// resetJobRetryCount resets a job's retry count to 0
func (s *JobScheduler) resetJobRetryCount(job *Job) error {
	return s.saveJobRetryCount(job, 0)
}

// Define job functions for security detail updates
// These wrappers avoid redeclaring functions that exist in other files
func securityDetailUpdateJob(conn *data.Conn) error {
	return securities.UpdateSecurityDetails(conn, false)
}

func securityCikUpdateJob(conn *data.Conn) error {
	return securities.UpdateSecurityCik(conn)
}

func simpleSecuritiesUpdateJob(conn *data.Conn) error {
	return securities.SimpleUpdateSecuritiesV2(conn)
}

// Wrapper for UpdateSectors to match JobFunc signature
func updateSectorsJob(conn *data.Conn) error {
	err := securities.UpdateSectors(context.Background(), conn) // Discard the statBlock
	return err                                                  // Return the error, if any
}

// Wrapper for yearly subscription credit update
func updateYearlySubscriptionCreditsJob(conn *data.Conn) error {
	return subscriptions.UpdateYearlySubscriptionCredits(conn)
}

// Wrapper for Stripe pricing sync
func syncPricingFromStripeJob(conn *data.Conn) error {
	return SyncPricingFromStripe(conn)
}

// checkPartialCoverageAndStartServices checks if OHLCV partial coverage is sufficient
// and starts screener and polygon websocket if they haven't been started yet
func checkPartialCoverageAndStartServices(conn *data.Conn) error {
	partialCoverageCheckMutex.Lock()
	defer partialCoverageCheckMutex.Unlock()

	// If both services are already started, no need to check
	if screenerStartedByPartialCoverage && polygonStartedByPartialCoverage {
		return nil
	}

	log.Printf("🔍 Checking OHLCV partial coverage (need 2 months back) - screener started: %v, polygon started: %v",
		screenerStartedByPartialCoverage, polygonStartedByPartialCoverage)

	// Check if partial coverage is sufficient
	hasCoverage, err := marketdata.CheckOHLCVPartialCoverage(conn)
	if err != nil {
		log.Printf("❌ Failed to check OHLCV partial coverage: %v", err)
		return err
	}

	if !hasCoverage {
		log.Printf("⏳ OHLCV partial coverage not yet sufficient - services remain blocked")
		return nil
	}

	log.Printf("✅ OHLCV partial coverage is sufficient - starting blocked services")

	// Start screener if not already started by partial coverage check
	if !screenerStartedByPartialCoverage {
		log.Printf("🚀 Starting screener updater due to sufficient partial coverage")
		err := screener.StartScreenerUpdaterLoop(conn)
		if err != nil {
			log.Printf("❌ Failed to start screener updater: %v", err)
		} else {
			screenerStartedByPartialCoverage = true
			log.Printf("✅ Screener updater started successfully")
		}
	}

	// Start polygon websocket if not already started by partial coverage check
	if !polygonStartedByPartialCoverage {
		log.Printf("🚀 Starting Polygon WebSocket due to sufficient partial coverage")
		err := startPolygonWebSocketInternal(conn)
		if err != nil {
			log.Printf("❌ Failed to start Polygon WebSocket: %v", err)
		} else {
			polygonStartedByPartialCoverage = true
			log.Printf("✅ Polygon WebSocket started successfully")
		}
	}

	return nil
}

// startPolygonWebSocketInternal is the internal implementation for starting polygon websocket
func startPolygonWebSocketInternal(conn *data.Conn) error {
	polygonInitMutex.Lock()
	defer polygonInitMutex.Unlock()

	if polygonInitialized {
		log.Printf("⚠️ Polygon WebSocket already running")
		return nil
	}

	err := socket.StartPolygonWS(conn, useBS, true)
	if err != nil {
		return err
	}
	polygonInitialized = true
	return nil
}

// Define all jobs and their schedules
var (
	JobList = []*Job{
		{
			Name:           "SyncPricingFromStripe",
			Function:       syncPricingFromStripeJob,
			Schedule:       []TimeOfDay{{Hour: 4, Minute: 0}}, // Run at 4:00 AM daily
			RunOnInit:      true,
			SkipOnWeekends: false, // Run every day to keep pricing up-to-date
			RetryOnFailure: true,
			MaxRetries:     3,
			RetryDelay:     1 * time.Minute,
		},
		{
			Name:           "UpdateSecurityTables",
			Function:       simpleSecuritiesUpdateJob,
			Schedule:       []TimeOfDay{{Hour: 21, Minute: 45}}, // Run at 9:45 PM - update ecurities table with currently listed tickers
			RunOnInit:      true,
			SkipOnWeekends: true,
			RetryOnFailure: true,
			MaxRetries:     2,
			RetryDelay:     1 * time.Minute,
		},
		{ // enable this before PR
			Name:           "UpdateAllOHLCV",
			Function:       marketdata.UpdateAllOHLCV,
			Schedule:       []TimeOfDay{{Hour: 21, Minute: 45}}, // Run at 9:45 PM - consolidates all OHLCV updates
			RunOnInit:      true,
			SkipOnWeekends: true,
			RetryOnFailure: true,
			MaxRetries:     100,
			RetryDelay:     1 * time.Minute,
		},
		// COMMENTED OUT: Aggregates initialization disabled, legacy code
		/*
			{
				Name:           "InitAggregates",
				Function:       initAggregates,
				Schedule:       []TimeOfDay{{Hour: 3, Minute: 56}}, // Run before market open
				RunOnInit:      true,
				SkipOnWeekends: true,
			},
		*/
		{
			Name:           "StartScreenerUpdater",
			Function:       startScreenerUpdater,               // Uses partial coverage guard
			Schedule:       []TimeOfDay{{Hour: 3, Minute: 58}}, // Run before market open
			RunOnInit:      true,
			SkipOnWeekends: true,
			RetryOnFailure: true,
			MaxRetries:     2,
			RetryDelay:     30 * time.Second,
		},
		//TODO: FIX THIS SHIT
		/*{
			Name:           "StartAlertLoop",

			Function:       startAlertLoop,
			Schedule:       []TimeOfDay{{Hour: 3, Minute: 57}}, // Run before market open
			RunOnInit:      true,
			SkipOnWeekends: true,
		},*/
		{
			Name:           "StartPolygonWebSocket",
			Function:       startPolygonWebSocket,
			Schedule:       []TimeOfDay{{Hour: 3, Minute: 58}}, // Run before market open
			RunOnInit:      true,
			SkipOnWeekends: true,
			RetryOnFailure: true,
			MaxRetries:     2,
			RetryDelay:     30 * time.Second,
		},
		{
			Name:           "UpdateSecurityDetails",
			Function:       securityDetailUpdateJob,
			Schedule:       []TimeOfDay{{Hour: 21, Minute: 0}}, // Run at 9:00 PM
			RunOnInit:      true,
			SkipOnWeekends: true,
			RetryOnFailure: true,
			MaxRetries:     2,
			RetryDelay:     1 * time.Minute,
		},
		{
			Name:           "StopServices",
			Function:       stopServicesJob,
			Schedule:       []TimeOfDay{{Hour: 20, Minute: 0}}, // Stop services at 8:00 PM
			RunOnInit:      false,
			SkipOnWeekends: true,
			RetryOnFailure: false, // Don't retry stop services
		},
		{
			Name:           "UpdateSectors",
			Function:       updateSectorsJob,                    // Use the new wrapper function
			Schedule:       []TimeOfDay{{Hour: 20, Minute: 15}}, // Run at 8:15 PM
			RunOnInit:      true,
			SkipOnWeekends: true,
			RetryOnFailure: true,
			MaxRetries:     2,
			RetryDelay:     1 * time.Minute,
		},
		{
			Name:           "UpdateSecurityCik",
			Function:       securityCikUpdateJob,
			Schedule:       []TimeOfDay{{Hour: 21, Minute: 30}}, // Run at 9:30 PM
			RunOnInit:      true,
			SkipOnWeekends: true,
			RetryOnFailure: true,
			MaxRetries:     2,
			RetryDelay:     1 * time.Minute,
		},
		{
			Name:           "StartWorkerMonitor",
			Function:       startWorkerMonitor,
			Schedule:       []TimeOfDay{{Hour: 3, Minute: 55}}, // Start before other services
			RunOnInit:      true,
			SkipOnWeekends: false, // Monitor should run 24/7
			RetryOnFailure: true,
			MaxRetries:     2,
			RetryDelay:     30 * time.Second,
		},
		{
			Name:           "UpdateYearlySubscriptionCredits",
			Function:       updateYearlySubscriptionCreditsJob,
			Schedule:       []TimeOfDay{{Hour: 4, Minute: 5}}, // Daily at 4:05 AM ET
			RunOnInit:      true,
			SkipOnWeekends: false,
			RetryOnFailure: true,
			MaxRetries:     2,
			RetryDelay:     1 * time.Minute,
		},
		{
			Name:     "CheckPartialCoverageAndStartServices",
			Function: checkPartialCoverageAndStartServices,
			Schedule: []TimeOfDay{
				{Hour: 0, Minute: 5},  // 12:05 AM
				{Hour: 1, Minute: 5},  // 1:05 AM
				{Hour: 2, Minute: 5},  // 2:05 AM
				{Hour: 3, Minute: 5},  // 3:05 AM
				{Hour: 4, Minute: 5},  // 4:05 AM
				{Hour: 5, Minute: 5},  // 5:05 AM
				{Hour: 6, Minute: 5},  // 6:05 AM
				{Hour: 7, Minute: 5},  // 7:05 AM
				{Hour: 8, Minute: 5},  // 8:05 AM
				{Hour: 9, Minute: 5},  // 9:05 AM
				{Hour: 10, Minute: 5}, // 10:05 AM
				{Hour: 11, Minute: 5}, // 11:05 AM
				{Hour: 12, Minute: 5}, // 12:05 PM
				{Hour: 13, Minute: 5}, // 1:05 PM
				{Hour: 14, Minute: 5}, // 2:05 PM
				{Hour: 15, Minute: 5}, // 3:05 PM
				{Hour: 16, Minute: 5}, // 4:05 PM
				{Hour: 17, Minute: 5}, // 5:05 PM
				{Hour: 18, Minute: 5}, // 6:05 PM
				{Hour: 19, Minute: 5}, // 7:05 PM
				{Hour: 20, Minute: 5}, // 8:05 PM
				{Hour: 21, Minute: 5}, // 9:05 PM
				{Hour: 22, Minute: 5}, // 10:05 PM
				{Hour: 23, Minute: 5}, // 11:05 PM
			}, // Run every hour at 5 minutes past to check coverage
			RunOnInit:      true,
			SkipOnWeekends: false, // Run every day to catch when coverage becomes sufficient
			RetryOnFailure: false, // Don't retry - will run again next hour
		},
	}
)

// isWeekend checks if the given time is on a weekend
func isWeekend(now time.Time) bool {
	weekday := now.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// NewScheduler creates a new job scheduler
func NewScheduler(conn *data.Conn) (*JobScheduler, error) {
	// Get the local timezone
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone: %w", err)
	}

	// Create the scheduler
	scheduler := &JobScheduler{
		Jobs:     JobList,
		Conn:     conn,
		Location: loc,
		StopChan: make(chan struct{}),
	}

	// Load job last run times from Redis
	scheduler.loadJobLastRunTimes()

	return scheduler, nil
}

// clearJobCache clears all job-related Redis cache entries
func clearJobCache(conn *data.Conn) error {
	ctx := context.Background()

	// Get all keys with the job last run prefix
	lastRunKeys, err := conn.Cache.Keys(ctx, jobLastRunKeyPrefix+"*").Result()
	if err == nil && len(lastRunKeys) > 0 {
		// Delete all last run keys
		err = conn.Cache.Del(ctx, lastRunKeys...).Err()
		if err != nil {
			return err
		}
	}

	// Get all keys with the job last completion prefix
	lastCompletionKeys, err := conn.Cache.Keys(ctx, jobLastCompletionKeyPrefix+"*").Result()
	if err != nil {
		return err
		// Log error getting job last completion keys
	} else if len(lastCompletionKeys) > 0 {
		// Delete all last completion keys
		err = conn.Cache.Del(ctx, lastCompletionKeys...).Err()
		if err != nil {
			return err
		}
	}

	// Get all keys with the job retry count prefix
	retryCountKeys, err := conn.Cache.Keys(ctx, jobRetryCountKeyPrefix+"*").Result()
	if err != nil {
		return err
		// Log error getting job retry count keys
	} else if len(retryCountKeys) > 0 {
		// Delete all retry count keys
		err = conn.Cache.Del(ctx, retryCountKeys...).Err()
		return err
	}
	return nil

}

// StartScheduler initializes and starts the job scheduler
func StartScheduler(conn *data.Conn) chan struct{} {
	// Clear job cache on server initialization
	if err := clearJobCache(conn); err != nil {
		log.Printf("Error clearing job cache: %v", err)
	}

	// NOTE: Removed early database state verification to avoid long blocking index builds on startup.
	// The state check is now executed after the OHLCV update job completes.

	scheduler, err := NewScheduler(conn)
	if err != nil {
		log.Fatalf("Failed to create scheduler: %v", err)
	}

	// Start the scheduler
	quit := scheduler.Start()
	return quit
}

// Start begins the job scheduler
func (s *JobScheduler) Start() chan struct{} {
	s.mutex.Lock()
	if s.IsRunning {
		s.mutex.Unlock()
		return s.StopChan
	}
	s.IsRunning = true
	s.mutex.Unlock()

	// Reload job last run times from Redis
	s.loadJobLastRunTimes()

	// Add 10-minute delay before starting scheduler operations
	log.Printf("⏰ Scheduler initialized - 5 seconds before starting job execution...")

	go func() {
		// Wait 5 seconds before starting scheduler operations
		select {
		case <-time.After(5 * time.Second):
			log.Printf("🚀 Starting scheduler operations after 5-second delay")
		case <-s.StopChan:
			log.Printf("⏹️ Scheduler stopped during startup delay")
			return
		}

		// Run jobs marked for initialization
		s.runInitJobs()

		// Start the Edgar Filings Service
		marketdata.StartEdgarFilingsService(s.Conn)
		go func() {
			for filing := range marketdata.NewFilingsChannel {
				socket.BroadcastGlobalSECFiling(filing)
			}
		}()

		// Start the ticker for regular job execution
		ticker := time.NewTicker(1 * time.Minute)

		// Create a separate ticker for queue status updates (every 5 minutes)
		queueStatusTicker := time.NewTicker(5 * time.Minute)

		for {
			select {
			case <-ticker.C:
				now := time.Now().In(s.Location)
				s.checkAndRunJobs(now)
			case <-s.StopChan:
				ticker.Stop()
				queueStatusTicker.Stop()
				// Stop alert loop and polygon websocket when scheduler stops
				stopAlertLoop()
				stopPolygonWebSocket()
				s.mutex.Lock()
				s.IsRunning = false
				s.mutex.Unlock()
				return
			}
		}
	}()

	return s.StopChan
}

// runInitJobs runs all jobs that are marked to run on initialization, doesnt respect SkipOnWeekends
func (s *JobScheduler) runInitJobs() {
	for _, job := range s.Jobs {
		if job.RunOnInit {
			go s.executeJob(job, time.Now().In(s.Location))
		}
	}
}

// checkAndRunJobs examines all jobs and runs those that are scheduled for the current time
func (s *JobScheduler) checkAndRunJobs(now time.Time) {
	for _, job := range s.Jobs {
		if job.SkipOnWeekends && isWeekend(now) {
			continue
		}

		// Check if the job should run at this time
		shouldRun := s.shouldRunJob(job, now)
		if shouldRun {
			go s.executeJob(job, now)
		}

		// Check if there's a pending retry for this job
		if s.hasPendingRetry(job) {
			log.Printf("🔄 Found pending retry for job %s, executing immediately", job.Name)
			go s.executeJob(job, now)
		}
	}
}

// shouldRunJob determines if a job should run based on its schedule
func (s *JobScheduler) shouldRunJob(job *Job, now time.Time) bool {
	currentHour, currentMinute := now.Hour(), now.Minute()

	// If job is already running, don't run it again
	job.ExecutionMutex.Lock()
	if job.IsRunning {
		job.ExecutionMutex.Unlock()
		return false
	}
	job.ExecutionMutex.Unlock()

	// Check if the job should run at the current time
	for _, timeOfDay := range job.Schedule {
		if timeOfDay.Hour == currentHour && timeOfDay.Minute == currentMinute {
			return true
		}
	}

	// Get next scheduled time for this job
	nextTime := s.getNextScheduledTime(job, now)
	if nextTime == nil {
		return false
	}

	// Use LastCompletionTime if available, otherwise fall back to LastRun
	lastJobTime := job.LastRun
	if !job.LastCompletionTime.IsZero() {
		// If we have a completion time, use that instead of last run time
		lastJobTime = job.LastCompletionTime
	}

	// Check if we've passed a scheduled time since the last run/completion
	if !lastJobTime.IsZero() {
		lastTimeDate := lastJobTime.In(s.Location)
		lastTimeDay := time.Date(lastTimeDate.Year(), lastTimeDate.Month(), lastTimeDate.Day(), 0, 0, 0, 0, s.Location)
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.Location)

		// If it's a new day and we haven't run yet today, check if we missed any runs
		if lastTimeDay.Before(todayStart) {
			nowMinutes := currentHour*60 + currentMinute
			nextMinutes := nextTime.Hour*60 + nextTime.Minute

			// If current time is past the next scheduled time, run the job
			if nowMinutes >= nextMinutes {
				return true
			}
		}
	}

	return false
}

// hasPendingRetry checks if a job has a pending retry that should be executed
func (s *JobScheduler) hasPendingRetry(job *Job) bool {
	if !job.RetryOnFailure {
		return false
	}

	currentRetryCount := s.loadJobRetryCount(job)
	return currentRetryCount > 0 && currentRetryCount <= job.MaxRetries
}

// getNextScheduledTime returns the next time the job should run
func (s *JobScheduler) getNextScheduledTime(job *Job, now time.Time) *TimeOfDay {
	if len(job.Schedule) == 0 {
		return nil
	}

	// Convert schedules to minutes for easier comparison
	var scheduledMinutes []int
	for _, tod := range job.Schedule {
		scheduledMinutes = append(scheduledMinutes, tod.Hour*60+tod.Minute)
	}

	// Sort the minutes
	sort.Ints(scheduledMinutes)

	// Current time in minutes
	currentMinutes := now.Hour()*60 + now.Minute()

	// Find the next scheduled time
	for _, schedMin := range scheduledMinutes {
		if schedMin >= currentMinutes {
			return &TimeOfDay{
				Hour:   schedMin / 60,
				Minute: schedMin % 60,
			}
		}
	}

	// If we've passed all scheduled times for today, return the first one for tomorrow
	return &TimeOfDay{
		Hour:   scheduledMinutes[0] / 60,
		Minute: scheduledMinutes[0] % 60,
	}
}

// executeJob runs a job and updates its last run time
func (s *JobScheduler) executeJob(job *Job, now time.Time) {
	// Prevent concurrent execution of the same job
	job.ExecutionMutex.Lock()
	if job.IsRunning {
		job.ExecutionMutex.Unlock()
		log.Printf("📋 Job %s is already running, skipping this execution", job.Name)

		// Clear pending retry if job is already running to prevent infinite retry loop
		if job.RetryOnFailure {
			currentRetryCount := s.loadJobRetryCount(job)
			if currentRetryCount > 0 {
				log.Printf("🔄 Clearing pending retry for already running job %s", job.Name)
				if err := s.resetJobRetryCount(job); err != nil {
					log.Printf("⚠️ Error clearing retry count for running job %s: %v", job.Name, err)
				}
			}
		}
		return
	}
	job.IsRunning = true
	job.ExecutionMutex.Unlock()

	// Job execution variables
	jobName := job.Name
	startTime := time.Now()

	// Recover from panics to avoid scheduler crash
	defer func() {
		if rec := recover(); rec != nil {
			var err error
			switch x := rec.(type) {
			case error:
				err = fmt.Errorf("panic: %w", x)
			default:
				err = fmt.Errorf("panic: %v", x)
			}
			_ = alerts.LogCriticalAlert(err, jobName)
			log.Printf("❌ Job %s panicked: %v", jobName, err)
		}
	}()

	// Log job start
	log.Printf("🚀 Starting job: %s at %s", jobName, startTime.Format("2006-01-02 15:04:05"))

	// Execute job with retry logic
	err := s.executeJobWithRetry(job, startTime)

	// Calculate execution duration
	duration := time.Since(startTime).Round(time.Millisecond)

	// Update job status
	job.ExecutionMutex.Lock()
	job.IsRunning = false
	job.LastRun = now
	job.ExecutionMutex.Unlock()

	if err := s.saveJobLastRunTime(job); err != nil {
		log.Printf("❌ Error saving job last run time for %s: %v", job.Name, err)
	}

	// Handle completion logging based on execution result
	if err != nil {
		log.Printf("❌ Job %s FAILED after %v: %v", jobName, duration, err)
		_ = alerts.LogCriticalAlert(err, jobName)
		return
	}

	// Job completed successfully
	log.Printf("✅ Job %s completed successfully in %v", jobName, duration)

	// Reset retry count on successful completion
	if job.RetryOnFailure {
		if err := s.resetJobRetryCount(job); err != nil {
			log.Printf("⚠️ Error resetting retry count for %s: %v", job.Name, err)
		}
	}

	// Update completion time
	completionTime := time.Now()
	job.ExecutionMutex.Lock()
	job.LastCompletionTime = completionTime
	job.ExecutionMutex.Unlock()
	if err := s.saveJobLastCompletionTime(job); err != nil {
		log.Printf("❌ Error saving job completion time for %s: %v", job.Name, err)
	}
}

// executeJobWithRetry executes a job with retry logic if configured
func (s *JobScheduler) executeJobWithRetry(job *Job, startTime time.Time) error {
	jobName := job.Name
	currentRetryCount := s.loadJobRetryCount(job)

	// Execute the job
	err := job.Function(s.Conn)

	// If job succeeded or retry is not enabled, return immediately
	if err == nil || !job.RetryOnFailure {
		return err
	}

	// Job failed and retry is enabled
	log.Printf("❌ Job %s failed (attempt %d/%d): %v", jobName, currentRetryCount+1, job.MaxRetries+1, err)

	// Check if we've exceeded max retries
	if currentRetryCount >= job.MaxRetries {
		log.Printf("❌ Job %s exceeded maximum retries (%d), giving up", jobName, job.MaxRetries)
		return err
	}

	// Increment retry count
	currentRetryCount++
	if err := s.saveJobRetryCount(job, currentRetryCount); err != nil {
		log.Printf("⚠️ Error saving retry count for %s: %v", job.Name, err)
	}

	// Log retry attempt
	log.Printf("🔄 Scheduling retry for job %s in %v (attempt %d/%d)", jobName, job.RetryDelay, currentRetryCount, job.MaxRetries)

	// Schedule retry after delay
	go func() {
		select {
		case <-time.After(job.RetryDelay):
			// Check if scheduler is still running
			s.mutex.Lock()
			if !s.IsRunning {
				s.mutex.Unlock()
				log.Printf("⚠️ Scheduler stopped, cancelling retry for job %s", jobName)
				return
			}
			s.mutex.Unlock()

			// Execute retry
			log.Printf("🔄 Retrying job %s (attempt %d/%d)", jobName, currentRetryCount, job.MaxRetries)
			retryErr := s.executeJobWithRetry(job, startTime)
			if retryErr != nil {
				log.Printf("❌ Job %s retry failed (attempt %d/%d): %v", jobName, currentRetryCount, job.MaxRetries, retryErr)
			}
		case <-s.StopChan:
			log.Printf("⚠️ Scheduler stopped, cancelling retry for job %s", jobName)
		}
	}()

	// Return the original error for immediate logging and alerting
	return err
}

// COMMENTED OUT: initAggregates function disabled
/*
// initAggregates initializes the aggregates
func initAggregates(conn *data.Conn) error {
	if useBS {
		// Use synchronous initialization during startup to avoid race conditions
		socket.InitAggregatesAsync(conn)
	}
	return nil
}
*/

// startAlertLoop starts the alert loop if not already running
// TODO: Currently commented out - see JobList for related commented job
/*
func startAlertLoop(conn *data.Conn) error {
	alertsInitMutex.Lock()
	defer alertsInitMutex.Unlock()

	if !alertsInitialized {
		err := alerts.StartAlertLoop(conn)
		if err != nil {
			//log.Printf("Failed to start alert loop: %v", err)
			return err
		}
		alertsInitialized = true
	}
	// Log that alert loop is already running

	return nil
}
*/

// startScreenerUpdater starts the screener updater if partial coverage is sufficient
func startScreenerUpdater(conn *data.Conn) error {
	partialCoverageCheckMutex.Lock()
	defer partialCoverageCheckMutex.Unlock()

	// If already started by partial coverage check, skip
	if screenerStartedByPartialCoverage {
		log.Printf("⚠️ Screener updater already started by partial coverage check")
		return nil
	}

	// Check if OHLCV partial coverage is sufficient (2 months back)
	hasCoverage, err := marketdata.CheckOHLCVPartialCoverage(conn)
	if err != nil {
		log.Printf("❌ Failed to check OHLCV partial coverage for screener: %v", err)
		return err
	}

	if !hasCoverage {
		log.Printf("⚠️ Screener updater blocked - OHLCV partial coverage not yet sufficient (need 2 months back)")
		return nil
	}

	// Partial coverage is sufficient, start the screener updater
	log.Printf("🚀 Starting screener updater - OHLCV partial coverage is sufficient")
	err = screener.StartScreenerUpdaterLoop(conn)
	if err != nil {
		return err
	}

	screenerStartedByPartialCoverage = true
	return nil
}

// startPolygonWebSocket starts the Polygon WebSocket if partial coverage is sufficient
func startPolygonWebSocket(conn *data.Conn) error {
	partialCoverageCheckMutex.Lock()
	defer partialCoverageCheckMutex.Unlock()

	// If already started by partial coverage check, skip
	if polygonStartedByPartialCoverage {
		log.Printf("⚠️ Polygon WebSocket already started by partial coverage check")
		return nil
	}

	// Check if OHLCV partial coverage is sufficient (2 months back)
	hasCoverage, err := marketdata.CheckOHLCVPartialCoverage(conn)
	if err != nil {
		log.Printf("❌ Failed to check OHLCV partial coverage for Polygon WebSocket: %v", err)
		return err
	}

	if !hasCoverage {
		log.Printf("⚠️ Polygon WebSocket blocked - OHLCV partial coverage not yet sufficient (need 2 months back)")
		return nil
	}

	// Partial coverage is sufficient, start the Polygon WebSocket
	log.Printf("🚀 Starting Polygon WebSocket - OHLCV partial coverage is sufficient")
	err = startPolygonWebSocketInternal(conn)
	if err != nil {
		return err
	}

	polygonStartedByPartialCoverage = true
	return nil
}

// Stop function for alert loop
func stopAlertLoop() {
	alertsInitMutex.Lock()
	defer alertsInitMutex.Unlock()

	if alertsInitialized {
		alerts.StopAlertLoop()
		alertsInitialized = false
	}
}

// stopPolygonWebSocket stops the Polygon WebSocket if it's running
func stopPolygonWebSocket() {
	polygonInitMutex.Lock()
	defer polygonInitMutex.Unlock()

	if polygonInitialized {
		// if err := socket.StopPolygonWS(); err != nil {
		// 	//log.Printf("Failed to stop Polygon WebSocket: %v", err)
		// }
		_ = socket.StopPolygonWS() // Assign to blank identifier if error is intentionally ignored
		polygonInitialized = false
	}
}

// stopServicesJob stops alert loop and polygon websocket as a scheduled job
func stopServicesJob(_ *data.Conn) error {
	stopAlertLoop()
	stopPolygonWebSocket()
	stopWorkerMonitor()
	return nil
}

// startWorkerMonitor starts the worker monitoring service
func startWorkerMonitor(conn *data.Conn) error {
	workerMonitorMutex.Lock()
	defer workerMonitorMutex.Unlock()

	if workerMonitor == nil {
		workerMonitor = worker_monitor.NewWorkerMonitor(conn)
		workerMonitor.Start()
		log.Println("✅ Worker monitor service started")
	} else {
		log.Println("⚠️ Worker monitor already running")
	}

	return nil
}

// stopWorkerMonitor stops the worker monitoring service
func stopWorkerMonitor() {
	workerMonitorMutex.Lock()
	defer workerMonitorMutex.Unlock()

	if workerMonitor != nil {
		workerMonitor.Stop()
		workerMonitor = nil
		log.Println("✅ Worker monitor service stopped")
	}
}
