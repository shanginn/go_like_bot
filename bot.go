package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/creasty/defaults"
	"github.com/go-redis/redis/v8"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/yaml.v2"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"
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

	UpdateBotsTokens []string `yaml:"updateBotsTokens"`

	Redis struct {
		Address string `default:"localhost:6379" yaml:"address"`
		Password string `default:"" yaml:"password"`
		Database int `default:"0" yaml:"database"`
	} `yaml:"redis"`
}

type UpdateBots struct {
	bots []*tgbotapi.BotAPI
	current  uint64
}

func (s *UpdateBots) nextIndex() int {
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.bots)))
}

func (s *UpdateBots) getNextBot() *tgbotapi.BotAPI {
	return s.bots[s.nextIndex()]
}

func parseConfig() (*Config, error) {
	f, err := os.Open("/data/config.yml")

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

	if cfg.Bot.Token == "" {
		return nil, errors.New("bot.token is required in config")
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

func setupRedis(config *Config) *redis.Client {
	return redis.NewClient(&redis.Options {
		Addr:     config.Redis.Address,
		Password: config.Redis.Password,
		DB:       config.Redis.Database,
	})
}

func loginBot(token string, debug bool) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(token)

	if err != nil {
		log.Panic(err)
	}

	bot.Debug = debug

	log.Printf("Authorized on account %s", bot.Self.UserName)

	return bot
}

func setupWebhook(config *Config, bot *tgbotapi.BotAPI) (tgbotapi.UpdatesChannel, error) {
	u, _ := url.Parse("https://" + config.Bot.Domain + ":" + config.Bot.Port + "/" + bot.Token)

	_, err := bot.SetWebhook(tgbotapi.WebhookConfig{
		URL: u,
		MaxConnections: 100,
	})

	if err != nil {
		return nil, err
	}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		return nil, err
	}

	if info.LastErrorDate != 0 {
		log.Printf("Last time telegram callback failed: %s", info.LastErrorMessage)
	}

	return bot.ListenForWebhook("/" + bot.Token), nil
}

func main() {
	config, err := parseConfig()

	if err != nil {
		log.Fatalf("Error while reading config: %s", err.Error())
	}

	bot = loginBot(config.Bot.Token, config.Bot.Debug)
	rdb = setupRedis(config)
	updates, err := setupWebhook(config, bot)

	if err != nil {
		log.Fatalf("Error setup webhook: %s", err.Error())
	}

	go func() {
		err := http.ListenAndServeTLS(
			"0.0.0.0:"+config.Bot.Port,
			config.Bot.CertPath,
			config.Bot.KeyPath,
			nil,
		)

		if err != nil {
			log.Fatalf("Failed to start web server: %s", err.Error())
		}

		log.Print("Server is started")
	}()

	var updateBots UpdateBots

	for _, token := range config.UpdateBotsTokens {
		updateBots.bots = append(updateBots.bots, loginBot(token, config.Bot.Debug))
	}

	for update := range updates {
		if update.CallbackQuery != nil {
			sendLikeButtonMarkup(
				*updateBots.getNextBot(),
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
