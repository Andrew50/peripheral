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
	chatID                      int64
	isProdEnv                   bool
)

func InitTelegramUserNotificationBot() error {
	env := strings.ToLower(os.Getenv("ENVIRONMENT"))
	if env == "demo" || env == "prod" || env == "production" {
		isProdEnv = true
	} else {
		isProdEnv = false
		return nil
	}
	isProdEnv = true
	userNotificationBotToken := "7988152298:AAGatpFVJuCVYpv547XFoApwMXzrKeRqoa8"
	fmt.Println("Initializing Telegram bot with token:", userNotificationBotToken)
	chatID = -1002517629348
	var err error
	telegramUserNotificationBot, err = telebot.NewBot(telebot.Settings{
		Token:  userNotificationBotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}
	benTweetsBotToken := "8112187727:AAG9JxDFQlUfrRt8tyjR5yyY_8Wd9o9ehZU"
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
	fmt.Println("Sending Telegram message to chat ID:", chatID)
	recipient := telebot.ChatID(chatID)
	_, err := telegramUserNotificationBot.Send(recipient, msg)
	return err
}

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
	recipient := telebot.ChatID(chatID)
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
