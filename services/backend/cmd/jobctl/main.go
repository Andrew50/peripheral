package main

import (
	"backend/jobs"
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

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

// getJobLastRunKey returns the Redis key for storing a job's last run time
func getJobLastRunKey(jobName string) string {
	return "job:lastrun:" + jobName
}

func getJobLastCompletionKey(jobName string) string {
	return "job:lastcompletion:" + jobName
}

func listJobs() {
	// Create a new scheduler to get the job list
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := utils.InitConn(inContainer)
	defer cleanup()

	scheduler, err := jobs.NewScheduler(conn)
	if err != nil {
		fmt.Printf("Error creating scheduler: %v\n", err)
		return
	}

	// Create a table for output
	table := NewTableWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Schedule", "Skip Weekends", "Run On Init"})

	// Sort jobs by name for consistent output
	sortedJobs := make([]*jobs.Job, len(scheduler.Jobs))
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

func formatSchedule(schedule []jobs.TimeOfDay) string {
	times := make([]string, len(schedule))
	for i, t := range schedule {
		times[i] = fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
	}
	return strings.Join(times, ", ")
}

func getJobStatus(jobName string) {
	// Create a new scheduler to get the job list
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := utils.InitConn(inContainer)
	defer cleanup()

	scheduler, err := jobs.NewScheduler(conn)
	if err != nil {
		fmt.Printf("Error creating scheduler: %v\n", err)
		return
	}

	// Find the job
	var job *jobs.Job
	for _, j := range scheduler.Jobs {
		if j.Name == jobName {
			job = j
			break
		}
	}

	if job == nil {
		fmt.Printf("Job '%s' not found\n", jobName)
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
	conn, cleanup := utils.InitConn(inContainer)
	defer cleanup()

	scheduler, err := jobs.NewScheduler(conn)
	if err != nil {
		fmt.Printf("Error creating scheduler: %v\n", err)
		return
	}

	// Create a table for output
	table := NewTableWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Last Run", "Last Completion", "Is Running"})

	// Sort jobs by name for consistent output
	sortedJobs := make([]*jobs.Job, len(scheduler.Jobs))
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

func runJob(jobName string) {
	// Create a new scheduler to get the job list
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := utils.InitConn(inContainer)
	defer cleanup()

	scheduler, err := jobs.NewScheduler(conn)
	if err != nil {
		fmt.Printf("Error creating scheduler: %v\n", err)
		return
	}

	// Find the job
	var job *jobs.Job
	for _, j := range scheduler.Jobs {
		if j.Name == jobName {
			job = j
			break
		}
	}

	if job == nil {
		fmt.Printf("Job '%s' not found\n", jobName)
		return
	}

	// Run the job
	fmt.Printf("Running job '%s'...\n", job.Name)
	startTime := time.Now()

	// Get initial queue length to compare after job execution
	initialQueueLen, err := conn.Cache.LLen(context.Background(), "queue").Result()
	if err != nil {
		fmt.Printf("Warning: Could not get initial queue length: %v\n", err)
		initialQueueLen = 0
	}

	// Execute the job function
	err = job.Function(conn)

	duration := time.Since(startTime).Round(time.Millisecond)

	if err != nil {
		fmt.Printf("\nJob failed after %v: %v\n", duration, err)
		return
	} else {
		fmt.Printf("\nJob completed successfully in %v\n", duration)
	}

	// Update last run time
	job.LastRun = time.Now()
	// We can't access the unexported method directly, so we'll update Redis manually
	lastRunStr := job.LastRun.Format(time.RFC3339)
	err = conn.Cache.Set(context.Background(), getJobLastRunKey(job.Name), lastRunStr, 0).Err()
	if err != nil {
		fmt.Printf("Error saving last run time: %v\n", err)
	}

	// Check if the job added items to the queue
	currentQueueLen, err := conn.Cache.LLen(context.Background(), "queue").Result()
	if err != nil {
		fmt.Printf("Warning: Could not get current queue length: %v\n", err)
		return
	}

	// If new items were added to the queue, monitor them
	if currentQueueLen > initialQueueLen {
		fmt.Printf("\nDetected %d new task(s) in the queue. Monitoring worker logs...\n", currentQueueLen-initialQueueLen)

		// Get the queued items
		queueItems, err := conn.Cache.LRange(context.Background(), "queue", 0, currentQueueLen-1).Result()
		if err != nil {
			fmt.Printf("Error getting queue items: %v\n", err)
			return
		}

		// Extract task IDs for monitoring
		var taskIDs []string
		var taskFuncs []string
		for _, item := range queueItems {
			var queueArgs utils.QueueArgs
			if err := json.Unmarshal([]byte(item), &queueArgs); err != nil {
				fmt.Printf("Error parsing queue item: %v\n", err)
				continue
			}
			taskIDs = append(taskIDs, queueArgs.ID)
			taskFuncs = append(taskFuncs, queueArgs.Func)
		}

		// Print task information
		fmt.Println("\nQueued tasks:")
		for i, id := range taskIDs {
			fmt.Printf("  %d: %s (ID: %s)\n", i+1, taskFuncs[i], id)
		}

		// Monitor task status and wait for completion
		fmt.Println("\nWaiting for worker to process tasks...")
		allTasksSucceeded := monitorTasksAndWait(conn, taskIDs)

		// Only update the last completion time if all tasks succeeded
		if allTasksSucceeded {
			completionTime := time.Now()
			// Update last completion time
			lastCompletionStr := completionTime.Format(time.RFC3339)
			err = conn.Cache.Set(context.Background(), getJobLastCompletionKey(job.Name), lastCompletionStr, 0).Err()
			if err != nil {
				fmt.Printf("Error saving last completion time: %v\n", err)
			} else {
				fmt.Printf("\nAll queued tasks completed successfully. Updated last completion time to %s\n",
					completionTime.Format("2006-01-02 15:04:05"))
			}
		} else {
			fmt.Println("\nNot all tasks completed successfully. Last completion time was not updated.")
		}
	} else {
		// For directly executed jobs with no queued tasks, update completion time immediately
		completionTime := time.Now()
		lastCompletionStr := completionTime.Format(time.RFC3339)
		err = conn.Cache.Set(context.Background(), getJobLastCompletionKey(job.Name), lastCompletionStr, 0).Err()
		if err != nil {
			fmt.Printf("Error saving last completion time: %v\n", err)
		} else {
			fmt.Printf("Updated last completion time to %s\n", completionTime.Format("2006-01-02 15:04:05"))
		}
	}
}

// monitorTasksAndWait polls the status of tasks, displays their progress, and returns whether all tasks completed successfully
func monitorTasksAndWait(conn *utils.Conn, taskIDs []string) bool {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	completedTasks := make(map[string]bool)
	failedTasks := make(map[string]bool)
	startTime := time.Now()
	timeout := 5 * time.Minute

	// Store previous statuses to avoid printing duplicates
	previousStatuses := make(map[string]string)

	// Print header
	fmt.Println("\n=== TASK MONITORING ===")
	fmt.Printf("Started at: %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Monitoring %d task(s)\n", len(taskIDs))
	fmt.Println("-------------------------")

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
					fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
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
					fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, status)
					previousStatuses[taskID] = status
				}
			} else {
				// Try to parse as result object
				err = json.Unmarshal([]byte(statusJSON), &result)
				if err == nil {
					// Check if it has an error field
					if errMsg, ok := result["error"]; ok && errMsg != nil {
						newStatus := fmt.Sprintf("Failed: %v", errMsg)
						if previousStatuses[taskID] != newStatus {
							fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
							previousStatuses[taskID] = newStatus
						}
						failedTasks[taskID] = true
					} else {
						// Task completed successfully
						newStatus := "Completed successfully"
						if previousStatuses[taskID] != newStatus {
							fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
							previousStatuses[taskID] = newStatus
						}
						completedTasks[taskID] = true
					}
				} else {
					newStatus := fmt.Sprintf("Error parsing status: %v", err)
					if previousStatuses[taskID] != newStatus {
						fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
						previousStatuses[taskID] = newStatus
					}
				}
			}
		}

		// Check if all tasks are completed or failed
		if len(completedTasks)+len(failedTasks) == len(taskIDs) {
			fmt.Printf("\n=== MONITORING COMPLETE ===\n")
			fmt.Printf("Duration: %v\n", time.Since(startTime).Round(time.Millisecond))
			fmt.Printf("%d/%d tasks completed successfully.\n", len(completedTasks), len(taskIDs))

			// List failed tasks if any
			if len(failedTasks) > 0 {
				fmt.Println("\nFailed tasks:")
				for _, taskID := range taskIDs {
					if failedTasks[taskID] {
						fmt.Printf("  - %s (Status: %s)\n", taskID, previousStatuses[taskID])
					}
				}
			}

			// Return true only if all tasks completed successfully
			return len(completedTasks) == len(taskIDs)
		}

		if time.Since(startTime) > timeout {
			fmt.Printf("\n=== MONITORING TIMEOUT ===\n")
			fmt.Printf("Timeout after %v waiting for tasks to complete.\n", timeout)
			fmt.Printf("%d/%d tasks completed successfully.\n", len(completedTasks), len(taskIDs))

			// List incomplete and failed tasks
			if len(completedTasks) < len(taskIDs) {
				fmt.Println("\nIncomplete or failed tasks:")
				for _, taskID := range taskIDs {
					if !completedTasks[taskID] {
						fmt.Printf("  - %s (Last status: %s)\n", taskID, previousStatuses[taskID])
					}
				}
			}

			return false
		}
	}
	return false
}

// monitorTasks polls the status of tasks and displays their progress
func monitorTasks(conn *utils.Conn, taskIDs []string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	completedTasks := make(map[string]bool)
	startTime := time.Now()
	timeout := 5 * time.Minute

	// Store previous statuses to avoid printing duplicates
	previousStatuses := make(map[string]string)

	// Print header
	fmt.Println("\n=== TASK MONITORING ===")
	fmt.Printf("Started at: %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Monitoring %d task(s)\n", len(taskIDs))
	fmt.Println("-------------------------")

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
					fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
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
					fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, status)
					previousStatuses[taskID] = status
				}

				if status != "completed" && status != "error" {
					continue
				} else {
					completedTasks[taskID] = true
				}
			} else {
				// Try to parse as response object
				err = json.Unmarshal([]byte(statusJSON), &result)
				if err != nil {
					newStatus := fmt.Sprintf("Error parsing status: %v", err)
					if previousStatuses[taskID] != newStatus {
						fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, newStatus)
						previousStatuses[taskID] = newStatus
					}
					continue
				}

				// Check status field
				if status, ok := result["status"].(string); ok {
					if previousStatuses[taskID] != status {
						fmt.Printf("[%s] Task %s: %s\n", time.Now().Format("15:04:05"), taskID, status)
						previousStatuses[taskID] = status
					}

					if status == "completed" || status == "error" {
						fmt.Printf("\n=== TASK %s DETAILS ===\n", taskID)

						// If there's a result, print it
						if result, ok := result["result"]; ok {
							resultJSON, _ := json.MarshalIndent(result, "", "  ")
							fmt.Printf("Result:\n%s\n", string(resultJSON))
						}

						// If there's an error, print it
						if errMsg, ok := result["error"].(string); ok && errMsg != "" {
							fmt.Printf("Error: %s\n", errMsg)
						}

						fmt.Println("======================")
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
			fmt.Printf("\n=== MONITORING COMPLETE ===\n")
			fmt.Printf("All %d tasks completed in %v\n", len(taskIDs), duration)
			return
		}

		// Check for timeout
		if time.Since(startTime) > timeout {
			fmt.Printf("\n=== MONITORING TIMEOUT ===\n")
			fmt.Printf("Timeout after %v waiting for tasks to complete.\n", timeout)
			fmt.Printf("%d/%d tasks completed.\n", len(completedTasks), len(taskIDs))

			// List incomplete tasks
			if len(completedTasks) < len(taskIDs) {
				fmt.Println("\nIncomplete tasks:")
				for _, taskID := range taskIDs {
					if !completedTasks[taskID] {
						fmt.Printf("  - %s (Last status: %s)\n", taskID, previousStatuses[taskID])
					}
				}
			}

			return
		}
	}
}

func getQueueStatus() {
	// Create a connection
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := utils.InitConn(inContainer)
	defer cleanup()

	// Get the queue length
	queueLen, err := conn.Cache.LLen(context.Background(), "queue").Result()
	if err != nil {
		fmt.Printf("Error getting queue length: %v\n", err)
		return
	}

	// Get queue items
	queueItems, err := conn.Cache.LRange(context.Background(), "queue", 0, 9).Result()
	if err != nil {
		fmt.Printf("Error getting queue items: %v\n", err)
		return
	}

	fmt.Printf("Queue length: %d\n\n", queueLen)

	if len(queueItems) > 0 {
		fmt.Println("Recent queue items:")
		table := NewTableWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Function", "Arguments"})

		for i, item := range queueItems {
			var queueArgs utils.QueueArgs
			if err := json.Unmarshal([]byte(item), &queueArgs); err != nil {
				fmt.Printf("Error parsing queue item: %v\n", err)
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

		if queueLen > 10 {
			fmt.Printf("... and %d more items\n", queueLen-10)
		}

		// Add a hint about the monitor command
		fmt.Println("\nTip: Use 'jobctl monitor <task_id>' to monitor a specific task's execution.")
	} else {
		fmt.Println("Queue is empty")
	}
}

func monitorTask(taskID string) {
	// Create a connection
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := utils.InitConn(inContainer)
	defer cleanup()

	// Monitor a single task
	fmt.Printf("Monitoring task %s...\n", taskID)
	monitorTasks(conn, []string{taskID})
}

func printUsage() {
	fmt.Println("Usage: jobctl [command] [arguments]")
	fmt.Println("\nAvailable commands:")

	// Define commands
	commands := map[string]Command{
		"list": {
			usage:       "list",
			description: "List all available jobs",
			execute:     func(args []string) { listJobs() },
		},
		"run": {
			usage:       "run [job_name]",
			description: "Run a specific job",
			execute: func(args []string) {
				if len(args) < 1 {
					fmt.Println("Error: job name is required")
					printUsage()
					return
				}
				runJob(args[0])
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
			execute:     func(args []string) { getQueueStatus() },
		},
		"monitor": {
			usage:       "monitor [task_id]",
			description: "Monitor a specific task by ID",
			execute: func(args []string) {
				if len(args) < 1 {
					fmt.Println("Error: task ID is required")
					printUsage()
					return
				}
				monitorTask(args[0])
			},
		},
		"help": {
			usage:       "help",
			description: "Show this help message",
			execute:     func(args []string) { printUsage() },
		},
	}

	// Sort commands for consistent output
	var cmdNames []string
	for name := range commands {
		cmdNames = append(cmdNames, name)
	}
	sort.Strings(cmdNames)

	for _, name := range cmdNames {
		cmd := commands[name]
		fmt.Printf("  %-10s %s\n", cmd.usage, cmd.description)
	}
}

func main() {
	// Check if we're running in a container
	if os.Getenv("IN_CONTAINER") == "" {
		// If not explicitly set, default to false when running on host
		os.Setenv("IN_CONTAINER", "false")
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
			execute:     func(args []string) { listJobs() },
		},
		"run": {
			usage:       "run [job_name]",
			description: "Run a specific job",
			execute: func(args []string) {
				if len(args) < 1 {
					fmt.Println("Error: job name is required")
					printUsage()
					return
				}
				runJob(args[0])
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
			execute:     func(args []string) { getQueueStatus() },
		},
		"monitor": {
			usage:       "monitor [task_id]",
			description: "Monitor a specific task by ID",
			execute: func(args []string) {
				if len(args) < 1 {
					fmt.Println("Error: task ID is required")
					printUsage()
					return
				}
				monitorTask(args[0])
			},
		},
		"help": {
			usage:       "help",
			description: "Show this help message",
			execute:     func(args []string) { printUsage() },
		},
	}

	if command, ok := commands[cmd]; ok {
		command.execute(args)
	} else {
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
	}
}
