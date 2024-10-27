package alerts

import (
	"backend/utils"
	"encoding/json"
	"fmt"
    "context"
	"log"
	"time"
	"gopkg.in/telebot.v3"
)

const (
    ChatId  = -1002428678944
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
	log.Println("Telegram bot initialized successfully")
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


func writePriceMessage(alert Alert){
    var directionStr string
    if *alert.Direction {
        directionStr = "above"
    }else{
        directionStr = "below"
    }
    message := fmt.Sprintf("%s %s %f", alert.Ticker,directionStr,alert.Price)
    alert.Message = &message
}
func dispatchAlert(conn *utils.Conn, alert Alert) error {
    query := `
        INSERT INTO alertLogs (alertId, timestamp, securityId)
        VALUES ($1, $2, $3)
    `
    err := conn.DB.QueryRow(context.Background(),
        query,
        alert.AlertId,
        time.Now(),
    )
    if err != nil {
        log.Printf("Failed to log alert to database: %v", err)
        return fmt.Errorf("failed to log alert: %v", err)
    }
    SendMessageInternal(*alert.Message, ChatId)  //todo

/*    messageJSON, err := json.Marshal(wsMessage)
    if err != nil {
        log.Printf("Failed to marshal websocket message: %v", err)
        return fmt.Errorf("failed to marshal websocket message: %v", err)
    }*/

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
