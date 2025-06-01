package alerts

import (
	"backend/internal/services/socket"
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/telebot.v3"
)

const (
	ChatID = -1002428678944
)

var bot *telebot.Bot

// InitTelegramBot performs operations related to InitTelegramBot functionality.
func InitTelegramBot() error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Println("Warning: TELEGRAM_BOT_TOKEN environment variable not set.")
		// Depending on policy, you might return an error or allow the bot to fail initialization
		// return errors.New("TELEGRAM_BOT_TOKEN not set")
	}
	var err error

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

var dispatchAlert = func(alert Alert) error {
	//log.Printf("DEBUG: Dispatching alert: %+v", alert)
	////fmt.Println("dispatching alert", alert)
	alertMessage := writeAlertMessage(alert)
	timestamp := time.Now()
	err := SendTelegramMessage(alertMessage, ChatID)
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

	return nil
}
