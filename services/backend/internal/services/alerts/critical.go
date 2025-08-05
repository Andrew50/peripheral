package alerts

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// getCallerName returns the function name to use in logs.
// If a custom name is provided, it uses that; otherwise it detects the caller
// function name, skipping any functions within the alerts package itself.
func getCallerName(customName ...string) string {
	// If custom name provided, use it
	if len(customName) > 0 && customName[0] != "" {
		return customName[0]
	}

	// Otherwise use runtime detection
	// Skip frames: 0:getCallerName, 1:LogCriticalAlert, 2:actual caller
	pcs := make([]uintptr, 10)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.Function, "alerts.") { // skip any function within alerts package
			return frame.Function
		}
		if !more {
			break
		}
	}
	return "unknown"
}

// LogCriticalAlert sends a critical error message to Telegram.
//
// The function lazily initialises the Telegram bot (using InitTelegramBot) if it
// has not been initialised yet. It enriches the provided error with contextual
// information such as the runtime environment (e.g. stage, demo, prod) and a
// UTC timestamp. Any failure to initialise the bot or send the alert is logged
// and returned to the caller.
//
// An optional functionName can be provided to override automatic function name detection.
func LogCriticalAlert(err error, functionName ...string) error {
	if err == nil {
		return nil // nothing to report
	}

	// Detect development environment and skip sending critical alerts. This avoids
	// requiring Telegram credentials during local development.
	envValue := strings.ToLower(os.Getenv("ENVIRONMENT"))
	if envValue == "" || envValue == "dev" || envValue == "development" {
		// Still write the error to the local log for visibility with clear delimiters.
		timestamp := time.Now().UTC().Format(time.RFC3339)

		// Resolve the caller function name for better context in dev logs.
		callerFn := getCallerName(functionName...)

		log.Printf("\n"+
			"=========================================\n"+
			"🚨 CRITICAL ERROR (DEV ENVIRONMENT) 🚨\n"+
			"=========================================\n"+
			"Time: %s UTC\n"+
			"Function: %s\n"+
			"Error: %v\n"+
			"=========================================\n",
			timestamp, callerFn, err)
		return nil
	}

	// Ensure the bot is initialised. We defer to the existing helper in
	// dispatch.go so that we reuse the same global bot/chatID variables.
	if bot == nil {
		if initErr := InitTelegramBot(); initErr != nil {
			// If bot initialisation itself fails, log it locally and surface the error.
			log.Printf("LogCriticalAlert: failed to initialise telegram bot: %v", initErr)
			return fmt.Errorf("telegram bot init error: %w", initErr)
		}
	}

	// Determine current application environment. Use the same variable the DB
	// health-monitor relies on (ENVIRONMENT). Fallback to K8S_NAMESPACE which is
	// automatically populated in Kubernetes if ENVIRONMENT is not explicitly
	// set. Avoid other env names to keep consistency across components.
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = os.Getenv("K8S_NAMESPACE")
	}
	if env == "" {
		env = "unknown"
	}

	// Resolve the caller function name to give more context where the error originated.
	callerFn := getCallerName(functionName...)

	// Prepare the message.
	timestamp := time.Now().UTC().Format(time.RFC3339)
	msg := fmt.Sprintf("\u26A0\uFE0F *Critical Alert*\nEnvironment: %s\nTime: %s UTC\nFunction: %s\nError: %v", env, timestamp, callerFn, err)

	// Send the message.
	if sendErr := SendTelegramMessage(msg, chatID); sendErr != nil {
		log.Printf("LogCriticalAlert: failed to send telegram message: %v", sendErr)
		return fmt.Errorf("failed to send telegram message: %w", sendErr)
	}

	return nil
}
