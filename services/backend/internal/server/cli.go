package server

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type QueueArgs struct {
	ID   string      `json:"id"`
	Func string      `json:"func"`
	Args interface{} `json:"args"`
}

// TableWriter represents a structure for handling TableWriter data.
type TableWriter struct {
	headers []string
	rows    [][]string
	writer  *os.File
}

// NewTableWriter performs operations related to NewTableWriter functionality.
func NewTableWriter(writer *os.File) *TableWriter {
	return &TableWriter{
		writer: writer,
	}
}

func (t *TableWriter) SetHeader(headers []string) {
	t.headers = headers
}

func (t *TableWriter) Append(row []string) {
	t.rows = append(t.rows, row)
}

func (t *TableWriter) Render() {
	// Calculate column widths
	colWidths := make([]int, len(t.headers))
	for i, h := range t.headers {
		colWidths[i] = len(h)
	}

	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Print headers
	fmt.Fprint(t.writer, "| ")
	for i, h := range t.headers {
		fmt.Fprintf(t.writer, "%-*s | ", colWidths[i], h)
	}
	fmt.Fprintln(t.writer)

	// Print separator
	fmt.Fprint(t.writer, "| ")
	for i := range t.headers {
		for j := 0; j < colWidths[i]; j++ {
			fmt.Fprint(t.writer, "-")
		}
		fmt.Fprint(t.writer, " | ")
	}
	fmt.Fprintln(t.writer)

	// Print rows
	for _, row := range t.rows {
		fmt.Fprint(t.writer, "| ")
		for i, cell := range row {
			if i < len(colWidths) {
				fmt.Fprintf(t.writer, "%-*s | ", colWidths[i], cell)
			}
		}
		fmt.Fprintln(t.writer)
	}
}

// Command structure
type Command struct {
	usage       string
	description string
	execute     func(args []string)
}

func listJobs() {
	// Create a new scheduler to get the job list
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := data.InitConn(inContainer)
	defer cleanup()

	scheduler, err := NewScheduler(conn)
	if err != nil {
		////fmt.Printf("Error creating scheduler: %v\n", err)
		return
	}

	// Create a table for output
	table := NewTableWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Schedule", "Skip Weekends", "Run On Init"})

	// Sort jobs by name for consistent output
	sortedJobs := make([]*Job, len(scheduler.Jobs))
	copy(sortedJobs, scheduler.Jobs)
	sort.Slice(sortedJobs, func(i, j int) bool {
		return sortedJobs[i].Name < sortedJobs[j].Name
	})

	for _, job := range sortedJobs {
		scheduleStr := formatSchedule(job.Schedule)
		table.Append([]string{
			job.Name,
			scheduleStr,
			fmt.Sprintf("%t", job.SkipOnWeekends),
			fmt.Sprintf("%t", job.RunOnInit),
		})
	}

	table.Render()
}

func formatSchedule(schedule []TimeOfDay) string {
	if len(schedule) == 0 {
		return "Manual only"
	}

	times := make([]string, len(schedule))
	for i, t := range schedule {
		times[i] = fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
	}
	return strings.Join(times, ", ")
}

func getJobStatus(jobName string) {
	// Create a new scheduler to get the job list
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := data.InitConn(inContainer)
	defer cleanup()

	scheduler, err := NewScheduler(conn)
	if err != nil {
		////fmt.Printf("Error creating scheduler: %v\n", err)
		return
	}

	// Find the job
	var job *Job
	for _, j := range scheduler.Jobs {
		if j.Name == jobName {
			job = j
			break
		}
	}

	if job == nil {
		////fmt.Printf("Job '%s' not found\n", jobName)
		return
	}

	// Get last run time from Redis
	lastRunStr, err := conn.Cache.Get(context.Background(), getJobLastRunKey(job.Name)).Result()
	if err != nil {
		lastRunStr = "Never"
	}

	// Get last completion time from Redis
	lastCompletionStr, err := conn.Cache.Get(context.Background(), getJobLastCompletionKey(job.Name)).Result()
	if err != nil {
		lastCompletionStr = "Never"
	}

	// Create a table for output
	table := NewTableWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Last Run", "Last Completion", "Is Running", "Next Run"})

	// Calculate next run time
	nextRun := "Unknown"
	// We can't access the unexported method directly, so we'll skip this for now
	// A future enhancement could be to add an exported method to the JobScheduler

	table.Append([]string{
		job.Name,
		lastRunStr,
		lastCompletionStr,
		fmt.Sprintf("%t", job.IsRunning),
		nextRun,
	})

	table.Render()
}

func getAllJobsStatus() {
	// Create a new scheduler to get the job list
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := data.InitConn(inContainer)
	defer cleanup()

	scheduler, err := NewScheduler(conn)
	if err != nil {
		////fmt.Printf("Error creating scheduler: %v\n", err)
		return
	}

	// Create a table for output
	table := NewTableWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Last Run", "Last Completion", "Is Running"})

	// Sort jobs by name for consistent output
	sortedJobs := make([]*Job, len(scheduler.Jobs))
	copy(sortedJobs, scheduler.Jobs)
	sort.Slice(sortedJobs, func(i, j int) bool {
		return sortedJobs[i].Name < sortedJobs[j].Name
	})

	for _, job := range sortedJobs {
		// Get last run time from Redis
		lastRunStr, err := conn.Cache.Get(context.Background(), getJobLastRunKey(job.Name)).Result()
		if err != nil {
			lastRunStr = "Never"
		}

		// Get last completion time from Redis
		lastCompletionStr, err := conn.Cache.Get(context.Background(), getJobLastCompletionKey(job.Name)).Result()
		if err != nil {
			lastCompletionStr = "Never"
		}

		table.Append([]string{
			job.Name,
			lastRunStr,
			lastCompletionStr,
			fmt.Sprintf("%t", job.IsRunning),
		})
	}

	table.Render()
}

func runJob(jobName string) error {
	// Create a new scheduler to get the job list
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := data.InitConn(inContainer)
	defer cleanup()

	scheduler, err := NewScheduler(conn)
	if err != nil {
		////fmt.Printf("Error creating scheduler: %v\n", err)
		return err
	}

	// Find the job
	var job *Job
	for _, j := range scheduler.Jobs {
		if j.Name == jobName {
			job = j
			break
		}
	}

	if job == nil {
		////fmt.Printf("Job '%s' not found\n", jobName)
		return fmt.Errorf("Job not found")
	}

	// Run the job
	////fmt.Printf("Running job '%s'...\n", job.Name)
	//startTime := time.Now()

	// Get initial queue length to compare after job execution
	initialQueueLen, err := conn.Cache.LLen(context.Background(), "queue").Result()
	if err != nil {
		////fmt.Printf("Warning: Could not get initial queue length: %v\n", err)
		initialQueueLen = 0
	}

	// Execute the job function
	err = job.Function(conn)

	//duration := time.Since(startTime).Round(time.Millisecond)
	if err != nil {
		////fmt.Printf("\nJob failed after %v: %v\n", duration, err)
		return err
	}
	////fmt.Printf("\nJob completed successfully in %v\n", duration)

	// Update last run time
	job.LastRun = time.Now()
	// We can't access the unexported method directly, so we'll update Redis manually
	lastRunStr := job.LastRun.Format(time.RFC3339)
	err = conn.Cache.Set(context.Background(), getJobLastRunKey(job.Name), lastRunStr, 0).Err()
	if err != nil {
		return err
	}

	// Check if the job added items to the queue
	currentQueueLen, err := conn.Cache.LLen(context.Background(), "queue").Result()
	if err != nil {
		////fmt.Printf("Warning: Could not get current queue length: %v\n", err)
		return err
	}

	// If new items were added to the queue, monitor them
	if currentQueueLen > initialQueueLen {
		////fmt.Printf("\nDetected %d new task(s) in the queue. Monitoring worker logs...\n", currentQueueLen-initialQueueLen)

		// Get the queued items
		queueItems, err := conn.Cache.LRange(context.Background(), "queue", 0, currentQueueLen-1).Result()
		if err != nil {
			////fmt.Printf("Error getting queue items: %v\n", err)
			return err
		}

		// Extract task IDs for monitoring
		var taskIDs []string
		var taskFuncs []string
		for _, item := range queueItems {
			var queueArgs QueueArgs
			if err := json.Unmarshal([]byte(item), &queueArgs); err != nil {
				////fmt.Printf("Error parsing queue item: %v\n", err)
				continue
			}
			taskIDs = append(taskIDs, queueArgs.ID)
			_ = append(taskFuncs, queueArgs.Func)
		}

		// Print task information
		////fmt.Println("\nQueued tasks:")
		//for i, id := range taskIDs {
		////fmt.Printf("  %d: %s (ID: %s)\n", i+1, taskFuncs[i], id)
		//}

		// Monitor task status and wait for completion
		////fmt.Println("\nWaiting for worker to process tasks...")
		allTasksSucceeded := monitorTasksAndWait(conn, taskIDs)

		// Only update the last completion time if all tasks succeeded
		if allTasksSucceeded {
			completionTime := time.Now()
			// Update last completion time
			lastCompletionStr := completionTime.Format(time.RFC3339)
			err = conn.Cache.Set(context.Background(), getJobLastCompletionKey(job.Name), lastCompletionStr, 0).Err()
			if err != nil {
				return err
			}
		}
	} else {
		// For directly executed jobs with no queued tasks, update completion time immediately
		completionTime := time.Now()
		lastCompletionStr := completionTime.Format(time.RFC3339)
		err = conn.Cache.Set(context.Background(), getJobLastCompletionKey(job.Name), lastCompletionStr, 0).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

// monitorTasksAndWait polls the status of tasks, displays their progress, and returns whether all tasks completed successfully
func monitorTasksAndWait(conn *data.Conn, taskIDs []string) bool {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	completedTasks := make(map[string]bool)
	failedTasks := make(map[string]bool)
	startTime := time.Now()
	timeout := 5 * time.Minute

	// Store previous statuses to avoid printing duplicates
	previousStatuses := make(map[string]string)

	// Print header
	////fmt.Println("\n=== TASK MONITORING ===")
	////fmt.Printf("Started at: %s\n", startTime.Format("2006-01-02 15:04:05"))
	////fmt.Printf("Monitoring %d task(s)\n", len(taskIDs))
	////fmt.Println("-------------------------")

	for range ticker.C {
		for _, taskID := range taskIDs {
			if completedTasks[taskID] || failedTasks[taskID] {
				continue
			}

			// Get task status
			statusJSON, err := conn.Cache.Get(context.Background(), taskID).Result()
			if err != nil {
				newStatus := fmt.Sprintf("Error: %v", err)
				if previousStatuses[taskID] != newStatus {
					////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
					previousStatuses[taskID] = newStatus
				}
				continue
			}

			// Parse status
			var status string
			var result map[string]interface{}

			// First try to parse as simple string (for "queued" or "running" status)
			err = json.Unmarshal([]byte(statusJSON), &status)
			if err == nil {
				// It's a string status
				if previousStatuses[taskID] != status {
					////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, status)
					previousStatuses[taskID] = status
				}
			} else {
				// Try to parse as result object
				err = json.Unmarshal([]byte(statusJSON), &result)
				if err != nil {
					newStatus := fmt.Sprintf("Error parsing status: %v", err)
					if previousStatuses[taskID] != newStatus {
						////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
						previousStatuses[taskID] = newStatus
					}
					continue
				}

				// DEBUG: Print the structure of the result
				////fmt.Printf("\nDEBUG: Task data structure keys: %v\n", getKeys(result))
				//if resultObj, ok := result["result"].(map[string]interface{}); ok {
				////fmt.Printf("DEBUG: Result field keys: %v\n", getKeys(resultObj))
				//}

				// Check if it has an error field
				if errMsg, ok := result["error"]; ok && errMsg != nil {
					newStatus := fmt.Sprintf("Failed: %v", errMsg)
					if previousStatuses[taskID] != newStatus {
						////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
						previousStatuses[taskID] = newStatus
					}
					failedTasks[taskID] = true
				} else {
					// Task completed successfully
					newStatus := "Completed successfully"
					if previousStatuses[taskID] != newStatus {
						////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
						previousStatuses[taskID] = newStatus
					}
					completedTasks[taskID] = true
				}
			}
		}

		// Check if all tasks are completed or failed
		if len(completedTasks)+len(failedTasks) == len(taskIDs) {
			////fmt.Printf("\n=== MONITORING COMPLETE ===\n")
			////fmt.Printf("Duration: %v\n", time.Since(startTime).Round(time.Millisecond))
			////fmt.Printf("%d/%d tasks completed successfully.\n", len(completedTasks), len(taskIDs))

			// List failed tasks if any
			/*
				if len(failedTasks) > 0 {
					////fmt.Println("\nFailed tasks:")
					//for _, taskID := range taskIDs {
					//	if failedTasks[taskID] {
					//		// Debug output would go here
					//	}
					//}
				}*/

			// Return true only if all tasks completed successfully
			return len(completedTasks) == len(taskIDs)
		}

		if time.Since(startTime) > timeout {
			////fmt.Printf("\n=== MONITORING TIMEOUT ===\n")
			////fmt.Printf("Timeout after %v waiting for tasks to complete.\n", timeout)
			////fmt.Printf("%d/%d tasks completed successfully.\n", len(completedTasks), len(taskIDs))

			// List incomplete and failed tasks
			/*
				if len(completedTasks) < len(taskIDs) {
					////fmt.Println("\nIncomplete or failed tasks:")
					//for _, taskID := range taskIDs {
					//	if !completedTasks[taskID] {
					//		// Debug output would go here
					//	}
					//}
				}
			*/

			return false
		}
	}
	// This line should be unreachable due to the loop condition, but Go requires a return statement.
	return false // Added default return
}

// monitorTasks polls the status of tasks and displays their progress
func monitorTasks(conn *data.Conn, taskIDs []string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	completedTasks := make(map[string]bool)
	startTime := time.Now()
	timeout := 5 * time.Minute

	// Store previous statuses to avoid printing duplicates
	previousStatuses := make(map[string]string)

	// Print header
	////fmt.Println("\n=== TASK MONITORING ===")
	////fmt.Printf("Started at: %s\n", startTime.Format("2006-01-02 15:04:05"))
	////fmt.Printf("Monitoring %d task(s)\n", len(taskIDs))
	////fmt.Println("-------------------------")

	for range ticker.C {
		for _, taskID := range taskIDs {
			if completedTasks[taskID] {
				continue
			}

			// Get task status
			statusJSON, err := conn.Cache.Get(context.Background(), taskID).Result()
			if err != nil {
				newStatus := fmt.Sprintf("Error: %v", err)
				if previousStatuses[taskID] != newStatus {
					////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
					previousStatuses[taskID] = newStatus
				}
				continue
			}

			// Parse status
			var status string
			var result map[string]interface{}

			// First try to parse as simple string (for "queued" or "running" status)
			err = json.Unmarshal([]byte(statusJSON), &status)
			if err == nil {
				// Simple status
				if previousStatuses[taskID] != status {
					////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, status)
					previousStatuses[taskID] = status
				}

				if status != "completed" && status != "error" {
					continue
				}
				completedTasks[taskID] = true
			} else {
				// Try to parse as response object
				err = json.Unmarshal([]byte(statusJSON), &result)
				if err != nil {
					newStatus := fmt.Sprintf("Error parsing status: %v", err)
					if previousStatuses[taskID] != newStatus {
						////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
						previousStatuses[taskID] = newStatus
					}
					continue
				}

				// DEBUG: Print the structure of the result
				////fmt.Printf("\nDEBUG: Task data structure keys: %v\n", getKeys(result))
				//if resultObj, ok := result["result"].(map[string]interface{}); ok {
				////fmt.Printf("DEBUG: Result field keys: %v\n", getKeys(resultObj))
				//}

				// Check status field
				if status, ok := result["status"].(string); ok {
					if previousStatuses[taskID] != status {
						////fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, status)
						previousStatuses[taskID] = status
					}

					if status == "completed" || status == "error" {
						////fmt.Printf("\n=== TASK %s DETAILS ===\n", taskID)

						// If there's a result, print it
						//		if result, ok := result["result"]; ok {
						//			resultJSON, _ := json.MarshalIndent(result, "", "  ")
						////fmt.Printf("Result:\n%s\n", string(resultJSON))
						//		}

						// If there's an error, print it
						//			if errMsg, ok := result["error"].(string); ok && errMsg != "" {
						//			}

						// Display logs if available - check both in result and at root level
						var logs []interface{}

						// First check if logs are at the root level
						if rootLogs, ok := result["logs"].([]interface{}); ok && len(rootLogs) > 0 {
							logs = rootLogs
							////fmt.Printf("\nDEBUG: Found %d logs at root level\n", len(rootLogs))
						}

						// If no logs found at root level, try within result field
						if len(logs) == 0 {
							if resultMap, ok := result["result"].(map[string]interface{}); ok {
								if resultLogs, ok := resultMap["logs"].([]interface{}); ok && len(resultLogs) > 0 {
									logs = resultLogs
									////fmt.Printf("\nDEBUG: Found %d logs in result field\n", len(resultLogs))
								}
							}
						}

						if len(logs) > 0 {
							////fmt.Println("\n=== TASK LOGS ===")
							for i, logEntry := range logs {
								_ = i // Prevent unused error due to commented out debug line
								logMap, ok := logEntry.(map[string]interface{})
								if !ok {
									////fmt.Printf("DEBUG: Log entry %d is not a map: %v\n", i, logEntry)
									continue
								}

								timestamp, _ := logMap["timestamp"].(string)
								message, _ := logMap["message"].(string)
								level, _ := logMap["level"].(string)
								_ = level // Prevent unused error due to commented out debug line

								////fmt.Printf("DEBUG: Log %d - timestamp: %v, message: %v, level: %v\n", i, timestamp != "", message != "", level != "")

								if message != "" {
									// Format timestamp
									_ = timestamp // Assign to blank identifier
									//_ = shortTimestamp // Prevent unused error due to commented out debug line
									/*if len(timestamp) > 19 {
										shortTimestamp = timestamp[:19] // Get just the YYYY-MM-DDTHH:MM:SS part
									}*/

									////fmt.Printf("[%s][%s] %s\n", shortTimestamp, level, message)
								}
							}
							////fmt.Println("=================")
						}

						////fmt.Println("======================")
						completedTasks[taskID] = true
					} else {
						continue
					}
				} else {
					continue
				}
			}
		}

		if len(completedTasks) == len(taskIDs) {
			duration := time.Since(startTime).Round(time.Millisecond)
			_ = duration // Prevent unused error due to commented out debug line
			////fmt.Printf("\n=== MONITORING COMPLETE ===\n")
			////fmt.Printf("All %d tasks completed in %v\n", len(taskIDs), duration)
			return
		}

		// Check for timeout
		if time.Since(startTime) > timeout {
			////fmt.Printf("\n=== MONITORING TIMEOUT ===\n")
			////fmt.Printf("Timeout after %v waiting for tasks to complete.\n", timeout)
			////fmt.Printf("%d/%d tasks completed.\n", len(completedTasks), len(taskIDs))

			// List incomplete tasks
			/*if len(completedTasks) < len(taskIDs) {
				////fmt.Println("\nIncomplete tasks:")
				//for _, taskID := range taskIDs {
				//	if !completedTasks[taskID] {
				//		// Debug output would go here
				//	}
				//}
			}*/

			return
		}
	}
}

func getQueueStatus() {
	// Create a connection
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := data.InitConn(inContainer)
	defer cleanup()

	// Get the queue length
	_, err := conn.Cache.LLen(context.Background(), "queue").Result()
	if err != nil {
		////fmt.Printf("Error getting queue length: %v\n", err)
		return
	}

	// Get queue items
	queueItems, err := conn.Cache.LRange(context.Background(), "queue", 0, 9).Result()
	if err != nil {
		////fmt.Printf("Error getting queue items: %v\n", err)
		return
	}

	////fmt.Printf("Queue length: %d\n\n", queueLen)

	if len(queueItems) > 0 {
		////fmt.Println("Recent queue items:")
		table := NewTableWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Function", "Arguments"})

		for i, item := range queueItems {
			var queueArgs QueueArgs
			if err := json.Unmarshal([]byte(item), &queueArgs); err != nil {
				////fmt.Printf("Error parsing queue item: %v\n", err)
				continue
			}

			// Format arguments as JSON string
			argsJSON, _ := json.Marshal(queueArgs.Args)

			table.Append([]string{
				queueArgs.ID,
				queueArgs.Func,
				string(argsJSON),
			})

			if i >= 9 {
				break
			}
		}

		table.Render()

		// Add a hint about the monitor command
		////fmt.Println("\nTip: Use 'jobctl monitor <task_id>' to monitor a specific task's execution.")
	}
}

func monitorTask(taskID string) {
	// Create a connection
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := data.InitConn(inContainer)
	defer cleanup()

	// Monitor a single task
	////fmt.Printf("Monitoring task %s...\n", taskID)
	monitorTasks(conn, []string{taskID})
}

func createInvite(planName string, trialDays int) {
	// Create a connection
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := data.InitConn(inContainer)
	defer cleanup()

	// Create the invite
	invite, err := data.CreateInvite(conn, planName, trialDays)
	if err != nil {
		fmt.Printf("Error creating invite: %v\n", err)
		return
	}

	// Print the invite code and link
	fmt.Printf("Invite created successfully!\n")
	fmt.Printf("Link: https://peripheral.io/invite/%s\n", invite.Code)
	fmt.Printf("Plan Name: %s\n", invite.PlanName)
	fmt.Printf("Trial Days: %d\n", invite.TrialDays)
}

func printUsage() {
	////fmt.Println("Usage: jobctl [command] [arguments]")
	////fmt.Println("\nAvailable commands:")

	// Define commands
	commands := map[string]Command{
		"list": {
			usage:       "list",
			description: "List all available jobs",
			execute:     func(_ []string) { listJobs() },
		},
		"run": {
			usage:       "run [job_name]",
			description: "Run a specific job",
			execute: func(args []string) {
				if len(args) < 1 {
					////fmt.Println("Error: job name is required")
					printUsage()
					return
				}
				err := runJob(args[0])
				if err != nil {
					fmt.Printf("Error running job: %v\n", err)
				}
			},
		},
		"create-invite": {
			usage:       "create-invite [plan_name] [trial_days]",
			description: "Create a new invite code (trial_days defaults to 30)",
			execute: func(args []string) {
				if len(args) < 1 {
					fmt.Println("Error: plan name is required")
					fmt.Println("Usage: jobctl create-invite [plan_name] [trial_days]")
					fmt.Println("Example: jobctl create-invite pro 30")
					return
				}

				planName := args[0]
				trialDays := 30 // default

				if len(args) >= 2 {
					if days, err := fmt.Sscanf(args[1], "%d", &trialDays); err != nil || days != 1 {
						fmt.Printf("Error: invalid trial days '%s', using default of 30\n", args[1])
						trialDays = 30
					}
				}

				createInvite(planName, trialDays)
			},
		},
		"status": {
			usage:       "status [job_name]",
			description: "Get status of a specific job or all jobs",
			execute: func(args []string) {
				if len(args) > 0 {
					getJobStatus(args[0])
				} else {
					getAllJobsStatus()
				}
			},
		},
		"queue": {
			usage:       "queue",
			description: "Get status of the job queue",
			execute:     func(_ []string) { getQueueStatus() },
		},
		"monitor": {
			usage:       "monitor [task_id]",
			description: "Monitor a specific task by ID",
			execute: func(args []string) {
				if len(args) < 1 {
					////fmt.Println("Error: task ID is required")
					printUsage()
					return
				}
				monitorTask(args[0])
			},
		},
		"help": {
			usage:       "help",
			description: "Show this help message",
			execute:     func(_ []string) { printUsage() },
		},
	}

	// Sort commands for consistent output
	var cmdNames []string
	for name := range commands {
		cmdNames = append(cmdNames, name)
	}
	sort.Strings(cmdNames)

	for _, name := range cmdNames {
		_ = commands[name] // Assign to blank identifier
		////fmt.Printf("  %-10s %s\n", cmd.usage, cmd.description)
	}
}
func StartCLI() {
	// Check if we're running in a container
	if os.Getenv("IN_CONTAINER") == "" {
		// If not explicitly set, default to false when running on host
		err := os.Setenv("IN_CONTAINER", "false")
		if err != nil {
			fmt.Printf("Warning: Failed to set IN_CONTAINER environment variable: %v\n", err)
		}
	}

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Define commands
	commands := map[string]Command{
		"list": {
			usage:       "list",
			description: "List all available jobs",
			execute:     func(_ []string) { listJobs() },
		},
		"run": {
			usage:       "run [job_name]",
			description: "Run a specific job",
			execute: func(args []string) {
				if len(args) < 1 {
					////fmt.Println("Error: job name is required")a
					printUsage()
					return
				}
				err := runJob(args[0])
				if err != nil {
					fmt.Printf("Error running job: %v\n", err)
				}
			},
		},
		"create-invite": {
			usage:       "create-invite [plan_name] [trial_days]",
			description: "Create a new invite code (trial_days defaults to 30)",
			execute: func(args []string) {
				if len(args) < 1 {
					fmt.Println("Error: plan name is required")
					fmt.Println("Usage: jobctl create-invite [plan_name] [trial_days]")
					fmt.Println("Example: jobctl create-invite pro 30")
					return
				}

				planName := args[0]
				trialDays := 30 // default

				if len(args) >= 2 {
					if days, err := fmt.Sscanf(args[1], "%d", &trialDays); err != nil || days != 1 {
						fmt.Printf("Error: invalid trial days '%s', using default of 30\n", args[1])
						trialDays = 30
					}
				}

				createInvite(planName, trialDays)
			},
		},
		"status": {
			usage:       "status [job_name]",
			description: "Get status of a specific job or all jobs",
			execute: func(args []string) {
				if len(args) > 0 {
					getJobStatus(args[0])
				} else {
					getAllJobsStatus()
				}
			},
		},
		"queue": {
			usage:       "queue",
			description: "Get status of the job queue",
			execute:     func(_ []string) { getQueueStatus() },
		},
		"monitor": {
			usage:       "monitor [task_id]",
			description: "Monitor a specific task by ID",
			execute: func(args []string) {
				if len(args) < 1 {
					////fmt.Println("Error: task ID is required")
					printUsage()
					return
				}
				monitorTask(args[0])
			},
		},
		"help": {
			usage:       "help",
			description: "Show this help message",
			execute:     func(_ []string) { printUsage() },
		},
	}

	if command, ok := commands[cmd]; ok {
		command.execute(args)
	} else {
		////fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
	}
}
