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
)

var ctx = context.Background()

type Config struct {
	Bot struct {
		Token string `yaml:"token"`
	} `yaml:"bot"`

	Redis struct {
		Address string `default:"localhost:6379" yaml:"address"`
		Password string `default:"" yaml:"password"`
		Database int `default:"0" yaml:"database"`
	}
}

func getKeyboardMarkup(likesCount int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d 😂", likesCount), strconv.Itoa(likesCount)),
		),
	)
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

func main() {
	config, err := parseConfig()

	if err != nil {
		log.Fatalf("Error while reading config: %s", err.Error())
	}

	if config.Bot.Token == "" {
		log.Fatalf("bot.token is required in config!")
	}

	bot, err := tgbotapi.NewBotAPI(config.Bot.Token)

	rdb := redis.NewClient(&redis.Options{
		Addr: config.Redis.Address,
		Password: config.Redis.Password, // no password set
		DB:       config.Redis.Database,  // use default DB
	})

	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			key := strconv.Itoa(update.CallbackQuery.Message.MessageID)

			likesCount, err := rdb.Incr(ctx, key).Result()

			if err != nil {
				log.Printf("String to int convert failed %s", err.Error())

				likesCount = 0
			}

			replyMarkup := tgbotapi.NewEditMessageReplyMarkup(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				getKeyboardMarkup(int(likesCount)),
			)

			bot.Send(replyMarkup)
		}

		if update.ChannelPost != nil {
			replyMarkup := tgbotapi.NewEditMessageReplyMarkup(
				update.ChannelPost.Chat.ID,
				update.ChannelPost.MessageID,
				getKeyboardMarkup(0),
			)

			bot.Send(replyMarkup)
		}
	}
}
