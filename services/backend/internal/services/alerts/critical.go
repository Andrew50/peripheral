package alerts

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogCriticalAlert sends a critical error message to Telegram.
//
// The function lazily initialises the Telegram bot (using InitTelegramBot) if it
// has not been initialised yet. It enriches the provided error with contextual
// information such as the runtime environment (e.g. stage, demo, prod) and a
// UTC timestamp. Any failure to initialise the bot or send the alert is logged
// and returned to the caller.
func LogCriticalAlert(err error) error {
	if err == nil {
		return nil // nothing to report
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

	// Prepare the message.
	timestamp := time.Now().UTC().Format(time.RFC3339)
	msg := fmt.Sprintf("\u26A0\uFE0F *Critical Alert*\nEnvironment: %s\nTime: %s UTC\nError: %v", env, timestamp, err)

	// Send the message.
	if sendErr := SendTelegramMessage(msg, chatID); sendErr != nil {
		log.Printf("LogCriticalAlert: failed to send telegram message: %v", sendErr)
		return fmt.Errorf("failed to send telegram message: %w", sendErr)
	}

	return nil
}
