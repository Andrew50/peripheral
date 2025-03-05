package alerts

import (
	"backend/socket"
	"backend/utils"
	"context"
	"fmt"
	"log"
	"time"

	"gopkg.in/telebot.v3"
)

const (
	ChatID = -1002428678944
)

var bot *telebot.Bot
// InitTelegramBot performs operations related to InitTelegramBot functionality.
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
// SendTelegramMessage performs operations related to SendTelegramMessage functionality.
func SendTelegramMessage(msg string, chatID int64) {
	recipient := telebot.ChatID(chatID)
	_, err := bot.Send(recipient, msg)
	if err != nil {
		log.Printf("Failed to send message to chat ID %d: %v", chatID, err)
	}
}

func writeAlertMessage(alert Alert) string {
	if alert.AlertType == "algo" {
		return "Algo alert"
	}
	if alert.SecurityID == nil {
		log.Println("SecurityID is nil")
		return "SecurityID is missing"
	}
	if alert.AlertType == "setup" {
		if alert.Price == nil {
			log.Println("Price is nil for setup alert")
			return "Price is missing for setup alert"
		}
		return fmt.Sprintf("%s %f", *alert.Ticker, *alert.Price)
	} else if alert.AlertType == "price" {
		if alert.Price == nil || alert.Direction == nil {
			log.Println("Price or Direction is nil for price alert")
			return "Price or Direction is missing for price alert"
		}
		if *alert.Direction {
			return fmt.Sprintf("%s price above %f", *alert.Ticker, *alert.Price)
		} else {
			return fmt.Sprintf("%s price below %f", *alert.Ticker, *alert.Price)
		}
	} else if alert.AlertType == "algo" {
		//return fmt.Sprintf("%s %s", *alert.Ticker, *alert.AlgoName)
	}
	return ""
}

func dispatchAlert(conn *utils.Conn, alert Alert) error {
	fmt.Println("dispatching alert", alert)
	alertMessage := writeAlertMessage(alert)
	timestamp := time.Now()
	SendTelegramMessage(alertMessage, ChatID)
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

	_, err := conn.DB.Exec(context.Background(),
		query,
		alert.AlertID,
		timestamp,
		*alert.SecurityID,
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
	_, err = conn.DB.Exec(context.Background(), updateQuery, alert.AlertID)
	if err != nil {
		log.Printf("Failed to disable alert with ID %d: %v", alert.AlertID, err)
		return fmt.Errorf("failed to disable alert: %v", err)
	}
	RemoveAlert(alert.AlertID)

	return nil
}

/*type SendMessageArgs struct {
	Message string `json:"message"`
	ChatID  int64  `json:"chatID"`
}
// SendMessage performs operations related to SendMessage functionality.
func SendMessage(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SendMessageArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("incorrect args 3l5lfgkkcj %v", err)
	}
	SendTelegramMessage(args.Message, args.ChatID)
	return nil, nil
}*/

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
