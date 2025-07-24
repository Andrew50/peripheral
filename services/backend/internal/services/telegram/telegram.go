// Package telegram provides functionality for sending notifications via Telegram
package telegram

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

var (
	telegramUserNotificationBot *telebot.Bot
	telegramBenTweetsBot        *telebot.Bot
	userChatID                  int64
	benTweetsChatID             int64
	isProdEnv                   bool
)

// InitTelegramUserNotificationBot initializes the Telegram bot for user notifications
func InitTelegramUserNotificationBot() error {
	env := strings.ToLower(os.Getenv("ENVIRONMENT"))
	if env == "demo" || env == "prod" || env == "production" {
		isProdEnv = true
	} else {
		isProdEnv = false
		return nil
	}
	isProdEnv = true
	userNotificationBotToken := os.Getenv("TELEGRAM_USER_NOTIFICATION_BOT_TOKEN")
	if userNotificationBotToken == "" {
		return fmt.Errorf("TELEGRAM_USER_NOTIFICATION_BOT_TOKEN environment variable is required")
	}
	fmt.Println("Initializing Telegram bot with token:", userNotificationBotToken)
	userChatID = -1002517629348
	var err error
	telegramUserNotificationBot, err = telebot.NewBot(telebot.Settings{
		Token:  userNotificationBotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}
	benTweetsBotToken := os.Getenv("TELEGRAM_BEN_TWEETS_BOT_TOKEN")
	if benTweetsBotToken == "" {
		return fmt.Errorf("TELEGRAM_BEN_TWEETS_BOT_TOKEN environment variable is required")
	}
	chatID = -4940706341
	telegramBenTweetsBot, err = telebot.NewBot(telebot.Settings{
		Token:  benTweetsBotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}

	return nil
}

// SendTelegramUserUsageMessage sends a usage-related message to users via Telegram
func SendTelegramUserUsageMessage(msg string) error {
	if !isProdEnv {
		return nil
	}
	if telegramUserNotificationBot == nil {
		err := InitTelegramUserNotificationBot()
		if err != nil {
			return fmt.Errorf("failed to initialize Telegram bot: %w", err)
		}
	}
	fmt.Println("Sending Telegram message to chat ID:", userChatID)
	recipient := telebot.ChatID(userChatID)
	_, err := telegramUserNotificationBot.Send(recipient, msg)
	if err != nil {
		fmt.Println("Failed to send Telegram message:", err)
	}
	return err
}

// SendTelegramBenTweetsMessage sends tweet information to the Telegram channel
func SendTelegramBenTweetsMessage(tweetURL string, id string, msg string, image string) error {
	if !isProdEnv {
		return nil
	}
	if telegramBenTweetsBot == nil {
		err := InitTelegramUserNotificationBot()
		if err != nil {
			return fmt.Errorf("failed to initialize Telegram bot: %w", err)
		}
	}
	recipient := telebot.ChatID(benTweetsChatID)
	if i := strings.IndexByte(image, ','); i >= 0 {
		image = image[i+1:]
	}
	data, err := base64.StdEncoding.DecodeString(image)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}
	_, err = telegramBenTweetsBot.Send(recipient, tweetURL)
	if err != nil {
		return fmt.Errorf("failed to send url: %w", err)
	}
	photo := &telebot.Photo{File: telebot.FromReader(bytes.NewReader(data)), Caption: msg}
	_, err = telegramBenTweetsBot.Send(recipient, photo)
	if err != nil {
		return fmt.Errorf("failed to send photo: %w", err)
	}
	deepLink := fmt.Sprintf("https://x.com/intent/post?in_reply_to=%s&text=%s", id, url.QueryEscape(msg))
	_, err = telegramBenTweetsBot.Send(recipient, deepLink)
	return err
}
