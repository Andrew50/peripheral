package alerts

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gopkg.in/telebot.v3"
)

const (
	ChatId = -1002428678944
)

var bot *telebot.Bot

func InitTelegramBot() error {
	botToken := "7500247744:AAGNsmjWYfb97XzppT2E0_8qoArgxLOz7e0"
	var err error

	bot, err = telebot.NewBot(telebot.Settings{
		Token:  botToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}
	//log.Println("debug: Telegram bot initialized successfully")
	return err
}
func SendMessageInternal(msg string, chatID int64) {
	recipient := telebot.ChatID(chatID)
	_, err := bot.Send(recipient, msg)
	if err != nil {
		log.Printf("Failed to send message to chat ID %d: %v", chatID, err)
	}
}

type SendMessageArgs struct {
	Message string `json:"message"`
	ChatID  int64  `json:"chatID"`
}

func SendMessage(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SendMessageArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("incorrect args 3l5lfgkkcj %v", err)
	}
	SendMessageInternal(args.Message, args.ChatID)
	return nil, nil
}

func writeMessage(conn *utils.Conn, alert Alert) string {
	if alert.SecurityId == nil {
		log.Println("SecurityId is nil")
		return "SecurityId is missing"
	}
	ticker, err := utils.GetTicker(conn, *alert.SecurityId, time.Now())
	if err != nil {
		return fmt.Sprintf("error getting ticker: %v", err)
	}
	if alert.AlertType == "setup" {
		if alert.Price == nil {
			log.Println("Price is nil for setup alert")
			return "Price is missing for setup alert"
		}
		return fmt.Sprintf("%s %f", ticker, *alert.Price)
	} else if alert.AlertType == "price" {
		if alert.Price == nil || alert.Direction == nil {
			log.Println("Price or Direction is nil for price alert")
			return "Price or Direction is missing for price alert"
		}
		if *alert.Direction {
			return fmt.Sprintf("%s price above %f", ticker, *alert.Price)
		} else {
			return fmt.Sprintf("%s price below %f", ticker, *alert.Price)
		}
	} else if alert.AlertType == "algo" {
		//return fmt.Sprintf("%s %s", *alert.Ticker, *alert.AlgoName)
	}
	return ""
}
func dispatchAlert(conn *utils.Conn, alert Alert) error {
	fmt.Println("dispatching alert", alert)
	message := writeMessage(conn, alert)
	SendMessageInternal(message, ChatId) //todo
	query := `
        INSERT INTO alertLogs (alertId, timestamp, securityId)
        VALUES ($1, $2, $3)
    `

	_, err := conn.DB.Exec(context.Background(),
		query,
		alert.AlertId,
		time.Now(),
		alert.SecurityId,
	)
	if err != nil {
		log.Printf("Failed to log alert to database: %v", err)
		return fmt.Errorf("failed to log alert: %v", err)
	}

	// Disable the alert by setting its active status to false
	updateQuery := `
        UPDATE alerts
        SET active = false
        WHERE alertId = $1
    `
	_, err = conn.DB.Exec(context.Background(), updateQuery, alert.AlertId)
	if err != nil {
		log.Printf("Failed to disable alert with ID %d: %v", alert.AlertId, err)
		return fmt.Errorf("failed to disable alert: %v", err)
	}
	RemoveAlert(alert.AlertId)

	return nil
}

// func main() {
// 	botToken := "7500247744:AAGNsmjWYfb97XzppT2E0_8qoArgxLOz7e0"
// 	if botToken == "" {
// 		log.Fatal("TELEGRAM_BOT_TOKEN environment variable not set")
// 	}

// 	// Create a new bot instance
// 	bot, err := telebot.NewBot(telebot.Settings{
// 		Token:  botToken,
// 		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
// 	})
// 	if err != nil {
// 		log.Fatalf("Failed to create bot: %v", err)
// 	}

// 	// List of chat IDs to send messages to
// 	chatIDs := []int64{
// 		-1002428678944,
// 	}

// 	// The message you want to send
// 	messageText := "Hello, this is a test message from my Go backend!"

// 	// Send messages concurrently for speed
// 	for _, chatID := range chatIDs {
// 		go func(chatID int64) {
// 			recipient := telebot.ChatID(chatID)
// 			_, err := bot.Send(recipient, messageText)
// 			if err != nil {
// 				log.Printf("Failed to send message to chat ID %d: %v", chatID, err)
// 			} else {
// 				log.Printf("Message sent to chat ID %d", chatID)
// 			}
// 		}(chatID)
// 	}

// 	// Keep the program running to allow goroutines to finish
// 	select {}
// }
