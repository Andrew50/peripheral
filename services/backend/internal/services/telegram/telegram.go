package telegram

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

var (
	telegramBot *telebot.Bot
	chatID      int64
	isProdEnv   bool
)

func InitTelegramUserNotificationBot() error {
	env := strings.ToLower(os.Getenv("ENVIRONMENT"))
	if env == "demo" || env == "prod" || env == "production" {
		isProdEnv = true
	} else {
		isProdEnv = false
		return nil
	}
	botToken := "7988152298:AAGatpFVJuCVYpv547XFoApwMXzrKeRqoa8"
	fmt.Println("Initializing Telegram bot with token:", botToken)
	chatID = -1002517629348
	var err error
	telegramBot, err = telebot.NewBot(telebot.Settings{
		Token:  botToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}

	return nil
}

func SendTelegramUserUsageMessage(msg string) error {
	if !isProdEnv {
		return nil
	}
	if telegramBot == nil {
		err := InitTelegramUserNotificationBot()
		if err != nil {
			return fmt.Errorf("failed to initialize Telegram bot: %w", err)
		}
	}
	fmt.Println("Sending Telegram message to chat ID:", chatID)
	recipient := telebot.ChatID(chatID)
	_, err := telegramBot.Send(recipient, msg)
	return err
}
