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

// Define a simple table writer interface
type TableWriter struct {
	headers []string
	rows    [][]string
	writer  *os.File
}

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

func listJobs() {
	// Create a new scheduler to get the job list
	conn, cleanup := utils.InitConn(true)
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
	conn, cleanup := utils.InitConn(true)
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

	// Create a table for output
	table := NewTableWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Last Run", "Is Running", "Next Run"})

	// Calculate next run time
	nextRun := "Unknown"
	// We can't access the unexported method directly, so we'll skip this for now
	// A future enhancement could be to add an exported method to the JobScheduler

	table.Append([]string{
		job.Name,
		lastRunStr,
		fmt.Sprintf("%t", job.IsRunning),
		nextRun,
	})

	table.Render()
}

func getAllJobsStatus() {
	// Create a new scheduler to get the job list
	conn, cleanup := utils.InitConn(true)
	defer cleanup()

	scheduler, err := jobs.NewScheduler(conn)
	if err != nil {
		fmt.Printf("Error creating scheduler: %v\n", err)
		return
	}

	// Create a table for output
	table := NewTableWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Last Run", "Is Running"})

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

		table.Append([]string{
			job.Name,
			lastRunStr,
			fmt.Sprintf("%t", job.IsRunning),
		})
	}

	table.Render()
}

func runJob(jobName string) {
	// Create a new scheduler to get the job list
	conn, cleanup := utils.InitConn(true)
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

	err = job.Function(conn)

	duration := time.Since(startTime).Round(time.Millisecond)

	if err != nil {
		fmt.Printf("\nJob failed after %v: %v\n", duration, err)
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
}

func getQueueStatus() {
	// Create connection
	conn, cleanup := utils.InitConn(true)
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
	} else {
		fmt.Println("Queue is empty")
	}
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
