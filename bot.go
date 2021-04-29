package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strconv"
)

var ctx = context.Background()
var bot *tgbotapi.BotAPI
var rdb *redis.Client

type BotConfig struct {
	Token string
	Debug bool
}

type RedisConfig struct {
	Address  string
	Password string
	Database int
}

type Config struct {
	Bot BotConfig
	Redis RedisConfig
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
	if config.Bot.Token == "" {
		log.Fatalln("bot.token is required in config!")
	}

	var err error

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

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			sendLikeButtonMarkup(
				*bot,
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				incLikesCount(update.CallbackQuery.Message.MessageID),
			)
		}

		if update.ChannelPost != nil {
			sendLikeButtonMarkup(
				*bot,
				update.ChannelPost.Chat.ID,
				update.ChannelPost.MessageID,
				0,
			)
		}
	}
}
