package telegram

import (
	"log"
	"time"

	"gopkg.in/telebot.v3"
)

var bot *telebot.Bot

func InitBot() error {
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
func SendMessage(msg string, chatID int64) {
	recipient := telebot.ChatID(chatID)
	_, err := bot.Send(recipient, msg)
	if err != nil {
		log.Printf("Failed to send message to chat ID %d: %v", chatID, err)
	}
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
