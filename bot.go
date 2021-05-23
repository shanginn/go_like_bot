package main

import (
	"context"
	"fmt"
	"github.com/creasty/defaults"
	"github.com/go-redis/redis/v8"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strconv"
	"net/http"
)

var ctx = context.Background()
var bot *tgbotapi.BotAPI
var rdb *redis.Client

type Config struct {
	Bot struct {
		Token string `yaml:"token"`
		Debug bool `default:"false" yaml:"debug"`
		Domain string `yaml:"domain"`
		Port string `yaml:"port"`
		CertPath string `yaml:"certPath"`
		KeyPath string `yaml:"keyPath"`
	} `yaml:"bot"`

	Redis struct {
		Address string `default:"localhost:6379" yaml:"address"`
		Password string `default:"" yaml:"password"`
		Database int `default:"0" yaml:"database"`
	}
}

func parseConfig() (*Config, error) {
	f, err := os.Open("config.yml")

	if err != nil {
		return nil, err
	}

	defer f.Close()

	cfg := &Config{}

	if err := defaults.Set(cfg); err != nil {
		return nil, err
	}

	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getKeyboardMarkup(likesCount int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%d 😂", likesCount),
				strconv.FormatInt(likesCount, 10),
			),
		),
	)
}

func getLikeButtonMarkup(ChatId int64, MessageID int, likesCount int64) tgbotapi.EditMessageReplyMarkupConfig {
	return tgbotapi.NewEditMessageReplyMarkup(
		ChatId,
		MessageID,
		getKeyboardMarkup(likesCount),
	)
}

func sendLikeButtonMarkup(bot tgbotapi.BotAPI, ChatId int64, MessageID int, likesCount int64) {
	_, err := bot.Send(getLikeButtonMarkup(
		ChatId,
		MessageID,
		likesCount,
	))

	if err != nil {
		log.Println(err.Error())
	}
}

func incLikesCount(messageId int) int64 {
	key := strconv.Itoa(messageId)

	likesCount, err := rdb.Incr(ctx, key).Result()

	if err != nil {
		log.Printf("String to int convert failed %s", err.Error())

		likesCount = 0
	}

	return likesCount
}

func main() {
	config, err := parseConfig()

	if err != nil {
		log.Fatalf("Error while reading config: %s", err.Error())
	}

	if config.Bot.Token == "" {
		log.Fatalln("bot.token is required in config!")
	}

	bot, err = tgbotapi.NewBotAPI(config.Bot.Token)

	if err != nil {
		log.Panic(err)
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     config.Redis.Address,
		Password: config.Redis.Password,
		DB:       config.Redis.Database,
	})

	bot.Debug = config.Bot.Debug

	log.Printf("Authorized on account %s", bot.Self.UserName)

	_, err = bot.SetWebhook(tgbotapi.NewWebhookWithCert(
		"https://" + config.Bot.Domain + ":" + config.Bot.Port + "/"+bot.Token,
		config.Bot.CertPath,
	))

	if err != nil {
		log.Fatal(err)
	}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}

	if info.LastErrorDate != 0 {
		log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
	}

	updates := bot.ListenForWebhook("/" + bot.Token)
	go http.ListenAndServeTLS(
		config.Bot.Domain + ":" + config.Bot.Port,
		config.Bot.CertPath,
		config.Bot.KeyPath,
		nil,
	)

	for update := range updates {
		log.Printf("%+v\n", update)
	}

	// u := tgbotapi.NewUpdate(0)
	// u.Timeout = 60

	// updates, err := bot.GetUpdatesChan(u)

	// for update := range updates {
	// 	if update.CallbackQuery != nil {
	// 		sendLikeButtonMarkup(
	// 			*bot,
	// 			update.CallbackQuery.Message.Chat.ID,
	// 			update.CallbackQuery.Message.MessageID,
	// 			incLikesCount(update.CallbackQuery.Message.MessageID),
	// 		)
	// 	}

	// 	if update.ChannelPost != nil {
	// 		sendLikeButtonMarkup(
	// 			*bot,
	// 			update.ChannelPost.Chat.ID,
	// 			update.ChannelPost.MessageID,
	// 			0,
	// 		)
	// 	}
	// }
}
