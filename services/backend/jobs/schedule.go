package jobs

import (
	"backend/alerts"
	"backend/socket"
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var useBS = true //alerts, securityUpdate, marketMetrics, sectorUpdate

var (
	polygonInitialized bool
	polygonInitMutex   sync.Mutex
	alertsInitialized  bool
	alertsInitMutex    sync.Mutex
)

// JobFunc represents a function that can be executed as a job
type JobFunc func(conn *utils.Conn) error

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
}

// JobScheduler manages and executes jobs
type JobScheduler struct {
	Jobs      []*Job
	Conn      *utils.Conn
	Location  *time.Location
	StopChan  chan struct{}
	IsRunning bool
	mutex     sync.Mutex
}

// Redis key prefix for job last run times
const jobLastRunKeyPrefix = "job:lastrun:"
const jobLastCompletionKeyPrefix = "job:lastcompletion:"

// getJobLastRunKey returns the Redis key for storing a job's last run time
func getJobLastRunKey(jobName string) string {
	return jobLastRunKeyPrefix + jobName
}

// getJobLastCompletionKey returns the Redis key for storing a job's last completion time
func getJobLastCompletionKey(jobName string) string {
	return jobLastCompletionKeyPrefix + jobName
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
				fmt.Printf("Loaded last run time for job %s: %s\n", job.Name, lastRun.Format(time.RFC3339))
			} else {
				fmt.Printf("Error parsing last run time for job %s: %v\n", job.Name, err)
			}
		} else if err != redis.Nil {
			fmt.Printf("Error loading last run time for job %s: %v\n", job.Name, err)
		}

		// Get the last completion time from Redis
		lastCompletionStr, err := s.Conn.Cache.Get(ctx, getJobLastCompletionKey(job.Name)).Result()
		if err == nil && lastCompletionStr != "" {
			// Parse the timestamp
			lastCompletion, err := time.Parse(time.RFC3339, lastCompletionStr)
			if err == nil {
				job.LastCompletionTime = lastCompletion
				fmt.Printf("Loaded last completion time for job %s: %s\n", job.Name, lastCompletion.Format(time.RFC3339))
			} else {
				fmt.Printf("Error parsing last completion time for job %s: %v\n", job.Name, err)
			}
		} else if err != redis.Nil {
			fmt.Printf("Error loading last completion time for job %s: %v\n", job.Name, err)
		}
	}
}

// saveJobLastRunTime saves a job's last run time to Redis
func (s *JobScheduler) saveJobLastRunTime(job *Job) {
	ctx := context.Background()

	// Store the last run time in Redis
	lastRunStr := job.LastRun.Format(time.RFC3339)
	err := s.Conn.Cache.Set(ctx, getJobLastRunKey(job.Name), lastRunStr, 0).Err()
	if err != nil {
		fmt.Printf("Error saving last run time for job %s: %v\n", job.Name, err)
	}
}

// saveJobLastCompletionTime saves a job's last completion time to Redis
func (s *JobScheduler) saveJobLastCompletionTime(job *Job) {
	ctx := context.Background()

	// Store the last completion time in Redis
	lastCompletionStr := job.LastCompletionTime.Format(time.RFC3339)
	err := s.Conn.Cache.Set(ctx, getJobLastCompletionKey(job.Name), lastCompletionStr, 0).Err()
	if err != nil {
		fmt.Printf("Error saving last completion time for job %s: %v\n", job.Name, err)
	}
}

// Define job functions for security detail updates
// These wrappers avoid redeclaring functions that exist in other files
func securityDetailUpdateJob(conn *utils.Conn) error {
	// Call the actual function from securitiesTable.go
	fmt.Println("Starting security details update - updating logos and icons...")
	return updateSecurityDetails(conn, true)
}

func securityCikUpdateJob(conn *utils.Conn) error {
	// We call the function from securities.go
	fmt.Println("Starting security CIK update - linking tickers to SEC identifiers...")
	return updateSecurityCik(conn)
}

func simpleSecuritiesUpdateJob(conn *utils.Conn) error {
	// We call the function from securities.go
	fmt.Println("Starting securities update - refreshing security data...")
	return simpleUpdateSecurities(conn)
}

// Helper function for pushJournals which returns void in journals.go
func pushJournalsJob(conn *utils.Conn, year int, month time.Month, day int) error {
	// Call the original function which doesn't return an error
	fmt.Printf("Pushing journals for date %d-%02d-%02d...\n", year, month, day)
	pushJournals(conn, year, month, day)
	return nil
}

// Define all jobs and their schedules
var (
	JobList = []*Job{
		{
			Name:           "ClearWorkerQueue",
			Function:       clearWorkerQueue,
			Schedule:       []TimeOfDay{{Hour: 3, Minute: 55}}, // Run before market open
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
		/*{
			Name:           "UpdateDailyOHLCV",
			Function:       UpdateDailyOHLCV,
			Schedule:       []TimeOfDay{{Hour: 21, Minute: 45}}, // Run at 9:45 PM
			RunOnInit:      true,
			SkipOnWeekends: true,
		},*/
		{
			Name:           "InitAggregates",
			Function:       initAggregates,
			Schedule:       []TimeOfDay{{Hour: 3, Minute: 56}}, // Run before market open
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
		{
			Name:           "StartAlertLoop",
			Function:       startAlertLoop,
			Schedule:       []TimeOfDay{{Hour: 3, Minute: 57}}, // Run before market open
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
		{
			Name:           "StartPolygonWebSocket",
			Function:       startPolygonWebSocket,
			Schedule:       []TimeOfDay{{Hour: 3, Minute: 58}}, // Run before market open
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
		{
			Name:           "PushJournals",
			Function:       pushJournalsForToday,
			Schedule:       []TimeOfDay{{Hour: 4, Minute: 5}}, // Run at 4:05 AM, shortly after market open
			RunOnInit:      false,
			SkipOnWeekends: true,
		},
		{
			Name:           "StopServices",
			Function:       stopServicesJob,
			Schedule:       []TimeOfDay{{Hour: 20, Minute: 0}}, // Stop services at 8:00 PM
			RunOnInit:      false,
			SkipOnWeekends: true,
		},
		{
			Name:           "UpdateSectors",
			Function:       updateSectors,
			Schedule:       []TimeOfDay{{Hour: 20, Minute: 15}}, // Run at 8:15 PM
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
		{
			Name:           "SimpleUpdateSecurities",
			Function:       simpleSecuritiesUpdateJob,
			Schedule:       []TimeOfDay{{Hour: 20, Minute: 45}}, // Run at 8:45 PM
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
		{
			Name:           "UpdateMarketMetrics",
			Function:       updateMarketMetrics,
			Schedule:       []TimeOfDay{{Hour: 20, Minute: 30}, {Hour: 8, Minute: 0}}, // Run at 8:30 PM and 8:00 AM
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
		{
			Name:           "UpdateSecurityDetails",
			Function:       securityDetailUpdateJob,
			Schedule:       []TimeOfDay{{Hour: 21, Minute: 0}}, // Run at 9:00 PM
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
		{
			Name:           "UpdateSecurityCik",
			Function:       securityCikUpdateJob,
			Schedule:       []TimeOfDay{{Hour: 21, Minute: 30}}, // Run at 9:30 PM
			RunOnInit:      true,
			SkipOnWeekends: true,
		},
	}
)

// isWeekend checks if the given time is on a weekend
func isWeekend(now time.Time) bool {
	weekday := now.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// NewScheduler creates a new job scheduler
func NewScheduler(conn *utils.Conn) (*JobScheduler, error) {
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
func clearJobCache(conn *utils.Conn) {
	ctx := context.Background()

	// Get all keys with the job last run prefix
	lastRunKeys, err := conn.Cache.Keys(ctx, jobLastRunKeyPrefix+"*").Result()
	if err != nil {
		fmt.Printf("Error getting job last run keys: %v\n", err)
	} else {
		// Delete all last run keys
		if len(lastRunKeys) > 0 {
			err = conn.Cache.Del(ctx, lastRunKeys...).Err()
			if err != nil {
				fmt.Printf("Error deleting job last run keys: %v\n", err)
			} else {
				fmt.Printf("Cleared %d job last run entries from Redis\n", len(lastRunKeys))
			}
		}
	}

	// Get all keys with the job last completion prefix
	lastCompletionKeys, err := conn.Cache.Keys(ctx, jobLastCompletionKeyPrefix+"*").Result()
	if err != nil {
		fmt.Printf("Error getting job last completion keys: %v\n", err)
	} else {
		// Delete all last completion keys
		if len(lastCompletionKeys) > 0 {
			err = conn.Cache.Del(ctx, lastCompletionKeys...).Err()
			if err != nil {
				fmt.Printf("Error deleting job last completion keys: %v\n", err)
			} else {
				fmt.Printf("Cleared %d job last completion entries from Redis\n", len(lastCompletionKeys))
			}
		}
	}

	fmt.Println("Job cache cleared successfully")
}

// StartScheduler initializes and starts the job scheduler
func StartScheduler(conn *utils.Conn) chan struct{} {
	// Clear job cache on server initialization
	clearJobCache(conn)

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

	// Run jobs marked for initialization
	s.runInitJobs()

	// Print initial queue status
	s.printQueueStatus()

	// Start the Edgar Filings Service
	fmt.Printf("\n\nStarting EdgarFilingsService\n\n")
	utils.StartEdgarFilingsService(s.Conn)
	go func() {
		for filing := range utils.NewFilingsChannel {
			fmt.Printf("\n\nBroadcasting global SEC filing\n\n")
			socket.BroadcastGlobalSECFiling(filing)
		}
	}()

	// Start the ticker for regular job execution
	ticker := time.NewTicker(1 * time.Minute)

	// Create a separate ticker for queue status updates (every 5 minutes)
	queueStatusTicker := time.NewTicker(5 * time.Minute)

	go func() {
		for {
			select {
			case <-ticker.C:
				now := time.Now().In(s.Location)
				s.checkAndRunJobs(now)
			case <-queueStatusTicker.C:
				// Print queue status every 5 minutes
				s.printQueueStatus()
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

// runInitJobs runs all jobs that are marked to run on initialization
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
	}
}

// pushJournalsForToday pushes journals for the current day
func pushJournalsForToday(conn *utils.Conn) error {
	now := time.Now()
	year, month, day := now.Date()
	return pushJournalsJob(conn, year, month, day)
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
		return
	}
	job.IsRunning = true
	job.ExecutionMutex.Unlock()

	// Job execution variables
	jobName := job.Name
	startTime := time.Now()
	isQueued := false
	var taskID string
	var err error

	// Log job start
	fmt.Printf("\n=== JOB START: %s ===\n", jobName)
	fmt.Printf("Time: %s\n", now.Format("2006-01-02 15:04:05"))

	// Execute job: either queue it or run directly
	if strings.HasPrefix(jobName, "queue:") {
		// Queue the job for async execution
		funcName := strings.TrimPrefix(jobName, "queue:")
		taskID, err = utils.Queue(s.Conn, funcName, nil)
		isQueued = true
		if err != nil {
			log.Printf("Error queueing job %s: %v", jobName, err)
		}
	} else {
		// Execute the job directly
		err = job.Function(s.Conn)
	}

	// Update job status
	duration := time.Since(startTime).Round(time.Millisecond)
	job.ExecutionMutex.Lock()
	job.IsRunning = false
	job.LastRun = now
	job.ExecutionMutex.Unlock()
	s.saveJobLastRunTime(job)

	// Handle completion logging based on execution method and result
	if err != nil {
		// Job failed
		fmt.Printf("\n=== JOB FAILED: %s ===\n", jobName)
		fmt.Printf("Duration: %v\n", duration)
		fmt.Printf("Error: %v\n", err)
	} else if isQueued {
		// Job was queued
		fmt.Printf("\n=== JOB QUEUED: %s ===\n", jobName)
		fmt.Printf("Duration: %v\n", duration)
		fmt.Printf("Task ID: %s\n", taskID)

		// Monitor queued task for completion
		if taskID != "" {
			go s.monitorQueuedTask(job, taskID, startTime)
		}
	} else {
		// Job completed directly
		fmt.Printf("\n=== JOB COMPLETED: %s ===\n", jobName)
		fmt.Printf("Duration: %v\n", duration)

		// Update completion time
		completionTime := time.Now()
		job.ExecutionMutex.Lock()
		job.LastCompletionTime = completionTime
		job.ExecutionMutex.Unlock()
		s.saveJobLastCompletionTime(job)
	}

	// Print queue status
	s.printQueueStatus()
}

// monitorQueuedTask polls a queued task until it completes or times out
func (s *JobScheduler) monitorQueuedTask(job *Job, taskID string, startTime time.Time) {
	jobName := job.Name
	maxRetries := 30
	retryInterval := time.Second * 10
	taskSucceeded := false

	for i := 0; i < maxRetries; i++ {
		result, pollErr := utils.Poll(s.Conn, taskID)
		if pollErr == nil && result != nil {
			// Try to parse the result to check for errors
			var resultMap map[string]interface{}
			if err := json.Unmarshal(result, &resultMap); err == nil {
				// Check if the result contains an error field
				if errVal, ok := resultMap["error"]; ok && errVal != nil {
					fmt.Printf("\n=== JOB VERIFICATION FAILED: %s ===\n", jobName)
					fmt.Printf("Error: %v\n", errVal)
					break
				}

				// Check if the state is completed
				if state, ok := resultMap["state"]; ok && state == "completed" {
					// Task completed successfully
					taskSucceeded = true
					completionTime := time.Now()

					job.ExecutionMutex.Lock()
					job.LastCompletionTime = completionTime
					job.ExecutionMutex.Unlock()

					// Save the completion time to Redis
					s.saveJobLastCompletionTime(job)

					fmt.Printf("\n=== JOB COMPLETED: %s ===\n", jobName)
					fmt.Printf("Actual Completion Time: %s\n", completionTime.Format("2006-01-02 15:04:05"))
					fmt.Printf("Task ID: %s\n", taskID)
					fmt.Printf("Total Duration: %v\n", time.Since(startTime).Round(time.Millisecond))
					break
				}
			}
		}

		// If we've reached the last retry, log a timeout
		if i == maxRetries-1 && !taskSucceeded {
			fmt.Printf("\n=== JOB VERIFICATION TIMEOUT: %s ===\n", jobName)
			fmt.Printf("Task ID: %s\n", taskID)
			fmt.Printf("Timed out after %d retries\n", maxRetries)
		}

		// Wait before retrying
		time.Sleep(retryInterval)
	}
}

// printQueueStatus prints the current status of the Redis queue
func (s *JobScheduler) printQueueStatus() {
	// Get the queue length
	queueLen, err := s.Conn.Cache.LLen(context.Background(), "queue").Result()
	if err != nil {
		fmt.Printf("Error getting queue length: %v\n", err)
		return
	}

	fmt.Println("\n=== REDIS QUEUE STATUS ===")
	fmt.Printf("Queue length: %d\n", queueLen)

	// If there are items in the queue, print the first few
	if queueLen > 0 {
		// Get up to 10 items from the queue (without removing them)
		items, err := s.Conn.Cache.LRange(context.Background(), "queue", 0, 9).Result()
		if err != nil {
			fmt.Printf("Error getting queue items: %v\n", err)
			return
		}

		fmt.Println("Queue items (up to 10):")
		for i, item := range items {
			// Try to parse the JSON to extract the function name
			var queueArgs utils.QueueArgs
			if err := json.Unmarshal([]byte(item), &queueArgs); err != nil {
				fmt.Printf("  %d: [Error parsing item: %v]\n", i+1, err)
			} else {
				fmt.Printf("  %d: %s (ID: %s)\n", i+1, queueArgs.Func, queueArgs.ID)
			}
		}

		// If there are more items than we displayed
		if queueLen > 10 {
			fmt.Printf("  ... and %d more items\n", queueLen-10)
		}
	} else {
		fmt.Println("Queue is empty")
	}

	fmt.Println("=========================")
}

func updateSectors(conn *utils.Conn) error {
	fmt.Println("Starting sector update - organizing securities by sectors...")
	taskID, err := utils.Queue(conn, "update_sectors", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("error queueing sector update: %w", err)
	}

	// Monitor the task until completion
	fmt.Printf("Sector update task queued with ID: %s. Monitoring for completion...\n", taskID)

	maxRetries := 30
	retryInterval := time.Second * 10

	for i := 0; i < maxRetries; i++ {
		result, pollErr := utils.Poll(conn, taskID)
		if pollErr == nil && result != nil {
			// Parse the result to check status
			var resultMap map[string]interface{}
			if err := json.Unmarshal(result, &resultMap); err == nil {
				// Check if the result contains an error field
				if errVal, ok := resultMap["error"]; ok && errVal != nil {
					return fmt.Errorf("sector update failed: %v", errVal)
				}

				// Check if state is completed
				if state, ok := resultMap["state"]; ok {
					if state == "completed" {
						fmt.Println("Sector update completed successfully")
						return nil
					} else if state == "failed" {
						return fmt.Errorf("sector update task failed")
					}
				}
			}
		}

		// If we've reached the last retry, log a timeout
		if i == maxRetries-1 {
			return fmt.Errorf("sector update task timed out after %d retries", maxRetries)
		}

		// Wait before retrying
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("sector update task monitoring failed")
}

func updateMarketMetrics(conn *utils.Conn) error {
	fmt.Println("Starting market metrics update - calculating market activity indicators...")
	_, err := utils.Queue(conn, "update_active", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("error queueing market metrics update: %w", err)
	}
	return nil
}

// clearWorkerQueue clears the worker queue to prevent backlog
func clearWorkerQueue(conn *utils.Conn) error {
	ctx := context.Background()

	err := conn.Cache.Del(ctx, "queue").Err()
	if err != nil {
		fmt.Println("Failed to clear worker queue:", err)
		return err
	}

	fmt.Println("Worker queue cleared successfully")
	return nil
}

// initAggregates initializes the aggregates
func initAggregates(conn *utils.Conn) error {
	if useBS {
		socket.InitAggregatesAsync(conn)
		fmt.Println("Aggregates initialized successfully")
	} else {
		fmt.Println("Skipping aggregates initialization (useBS is false)")
	}
	return nil
}

// startAlertLoop starts the alert loop if not already running
func startAlertLoop(conn *utils.Conn) error {
	if !useBS {
		fmt.Println("Skipping alert loop (useBS is false)")
		return nil
	}

	alertsInitMutex.Lock()
	defer alertsInitMutex.Unlock()

	if !alertsInitialized {
		err := alerts.StartAlertLoop(conn)
		if err != nil {
			fmt.Println("Failed to start alert loop:", err)
			return err
		}
		alertsInitialized = true
		fmt.Println("Alert loop started successfully")
	} else {
		fmt.Println("Alert loop already running")
	}

	return nil
}

// startPolygonWebSocket starts the Polygon WebSocket if not already running
func startPolygonWebSocket(conn *utils.Conn) error {
	polygonInitMutex.Lock()
	defer polygonInitMutex.Unlock()

	if !polygonInitialized {
		err := socket.StartPolygonWS(conn, useBS)
		if err != nil {
			log.Printf("Failed to start Polygon WebSocket: %v", err)
			return err
		}
		polygonInitialized = true
		fmt.Println("Polygon WebSocket started successfully")
	} else {
		fmt.Println("Polygon WebSocket already running")
	}

	return nil
}

// Stop function for alert loop
func stopAlertLoop() {
	alertsInitMutex.Lock()
	defer alertsInitMutex.Unlock()

	if alertsInitialized {
		alerts.StopAlertLoop()
		alertsInitialized = false
		fmt.Println("Alert loop stopped successfully")
	}
}

// stopPolygonWebSocket stops the Polygon WebSocket if it's running
func stopPolygonWebSocket() {
	polygonInitMutex.Lock()
	defer polygonInitMutex.Unlock()

	if polygonInitialized {
		if err := socket.StopPolygonWS(); err != nil {
			log.Printf("Failed to stop Polygon WebSocket: %v", err)
		} else {
			polygonInitialized = false
			fmt.Println("Polygon WebSocket stopped successfully")
		}
	}
}

// stopServicesJob stops alert loop and polygon websocket as a scheduled job
func stopServicesJob(conn *utils.Conn) error {
	fmt.Println("Stopping services for the night - alert loop and polygon websocket...")
	stopAlertLoop()
	stopPolygonWebSocket()
	return nil
}
