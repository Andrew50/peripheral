package alerts

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gopkg.in/telebot.v3"
)

var (
	bot    *telebot.Bot
	chatID int64
)

// InitTelegramBot performs operations related to InitTelegramBot functionality.
func InitTelegramBot() error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("Error: TELEGRAM_BOT_TOKEN environment variable is required.")
	}

	// Read chat ID from environment variable
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" {
		log.Fatal("Error: TELEGRAM_CHAT_ID environment variable is required.")
	}

	var err error
	chatID, err = strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Error: Invalid TELEGRAM_CHAT_ID format: %v", err)
	}

	bot, err = telebot.NewBot(telebot.Settings{
		Token:  botToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}
	////log.Println("debug: Telegram bot initialized successfully")
	return err
}

// SendTelegramMessage performs operations related to SendTelegramMessage functionality.
func SendTelegramMessage(msg string, chatID int64) error {
	recipient := telebot.ChatID(chatID)
	_, err := bot.Send(recipient, msg)
	return err
	//	if err != nil {
	//
	// log.Printf("Failed to send message to chat ID %d: %v", chatID, err)
	// }
}

func writeAlertMessage(alert Alert) string {
	if alert.AlertType == "algo" {
		return "Algo alert"
	}
	if alert.SecurityID == nil {
		//log.Println("SecurityID is nil")
		return "SecurityID is missing"
	}
	if alert.AlertType == "setup" {
		if alert.Price == nil {
			//log.Println("Price is nil for setup alert")
			return "Price is missing for setup alert"
		}
		return fmt.Sprintf("%s %f", *alert.Ticker, *alert.Price)
	} else if alert.AlertType == "price" {
		if alert.Price == nil || alert.Direction == nil {
			//log.Println("Price or Direction is nil for price alert")
			return "Price or Direction is missing for price alert"
		}
		if *alert.Direction {
			return fmt.Sprintf("%s price above %f", *alert.Ticker, *alert.Price)
		}
		return fmt.Sprintf("%s price below %f", *alert.Ticker, *alert.Price)
	} else if alert.AlertType == "algo" {
		return fmt.Sprintf("Algo alert triggered (ID: %d)", *alert.AlgoID)
	}
	return ""
}

func dispatchAlert(conn *data.Conn, alert Alert) error {
	//log.Printf("DEBUG: Dispatching alert: %+v", alert)
	////fmt.Println("dispatching alert", alert)
	alertMessage := writeAlertMessage(alert)
	timestamp := time.Now()
	err := SendTelegramMessage(alertMessage, chatID)
	if err != nil {
		return err
	}
	socket.SendAlertToUser(alert.UserID, socket.AlertMessage{
		AlertID:    alert.AlertID,
		Timestamp:  timestamp.Unix() * 1000,
		SecurityID: *alert.SecurityID,
		Message:    alertMessage,
		Channel:    "alert",
		Ticker:     *alert.Ticker,
	})
	query := `
        INSERT INTO alertLogs (alertId, timestamp, securityId)
        VALUES ($1, $2, $3)
    `

	_, err = conn.DB.Exec(context.Background(),
		query,
		alert.AlertID,
		timestamp,
		*alert.SecurityID,
	)
	if err != nil {
		//log.Printf("Failed to log alert to database: %v", err)
		return fmt.Errorf("failed to log alert: %v", err)
	}

	// Disable the alert by setting its active status to false
	updateQuery := `
        UPDATE alerts
        SET active = false
        WHERE alertId = $1
    `
	_, err = conn.DB.Exec(context.Background(), updateQuery, alert.AlertID)
	if err != nil {
		//log.Printf("Failed to disable alert with ID %d: %v", alert.AlertID, err)
		return fmt.Errorf("failed to disable alert: %v", err)
	}

	// Remove alert from memory and decrement counter
	if err := RemoveAlert(conn, alert.AlertID); err != nil {
		// Log the error but don't fail the dispatch since the alert has already been processed
		log.Printf("Warning: %v", err)
	}

	return nil
}
