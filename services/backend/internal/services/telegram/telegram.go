package telegram

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

var (
	telegramBot *telebot.Bot
	chatID      int64
	isDevEnv    bool
)

func InitTelegramUserNotificationBot() error {
	env := strings.ToLower(os.Getenv("ENVIRONMENT"))
	if env == "" || env == "dev" || env == "development" {
		isDevEnv = true
		log.Println("InitTelegramBot: development environment detected, skipping Telegram bot initialisation")
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
	if isDevEnv {
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
