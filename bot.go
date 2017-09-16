package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	PRICE_API_ENDPOINT = "https://coincap.io/page/%s"
	USERNAME_SEPARATOR = "@"
	BOT_NAME           = USERNAME_SEPARATOR + "coincap_prices_bot"
)

/* Commands */
const (
	QUOTE_COMMAND = "/quote"
	START_COMMAND = "/start"
	HELP_COMMAND  = "/help"
)

var (
	controllers = map[string]Controller{
		START_COMMAND: Start,
		QUOTE_COMMAND: Quote,
		HELP_COMMAND:  Help,
	}
)

/* Messages */
var (
	WELCOME_MESSAGE string = "Ask me for prices with /quote (ticker). Example: /quote BTC"
	HELP_MESSAGE    string = "Use me to get prices from https://coincap.io. Just type /quote (Ticker Symbol). For " +
		"example, /quote BTC."
)

type Controller func(*tgbotapi.BotAPI, tgbotapi.Update, []string)

func Start(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	reply(bot, update, WELCOME_MESSAGE)
}

func Help(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	reply(bot, update, HELP_MESSAGE)
}

func Quote(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	if len(arguments) < 1 {
		Help(bot, update, arguments)
		return
	}

	ticker := strings.ToUpper(arguments[0])
	url := fmt.Sprintf(PRICE_API_ENDPOINT, ticker)
	response, err := http.Get(url)
	if err != nil || response.StatusCode != 200 {
		reply(bot, update, fmt.Sprintf("Error retreiving %s price", ticker))
		return
	}

	if response.ContentLength == 2 {
		reply(bot, update, fmt.Sprintf("%s is not on https://coincap.io", ticker))
		return
	}
	var coinQuoteResponse map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&coinQuoteResponse)
	if err != nil {
		log.Println(err.Error())
		reply(bot, update, fmt.Sprintf("Error decoding response for %s", ticker))
		return
	}
	coinPriceUsd := coinQuoteResponse["price_usd"]
	quoteMessage := getQuoteFormat(coinPriceUsd)
	reply(bot, update,  fmt.Sprintf(quoteMessage, ticker, coinPriceUsd))

}

func getQuoteFormat(coinPriceUsd interface{}) string {
	var quoteMessage string
	switch coinPriceUsd.(type) {
	case float64, float32:
		if coinPriceUsd.(float64) < 0.99 {
			quoteMessage = "1 %s = USD$%.8f"
		} else {
			quoteMessage = "1 %s = USD$%.2f"
		}
	}

	return quoteMessage
}


func reply(bot *tgbotapi.BotAPI, update tgbotapi.Update, message string) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
	msg.ReplyToMessageID = update.Message.MessageID
	bot.Send(msg)
}

func routeCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if !update.Message.IsCommand() {
		command := update.Message.Text
		Quote(bot, update, []string{command})
	} else {
		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		parts := strings.Split(update.Message.Text, " ")
		if len(parts) < 1 {
			Help(bot, update, []string{})
			return
		}

		controllerName := parts[0]
		if strings.Contains(controllerName, BOT_NAME) {
			controllerName = strings.Split(controllerName, USERNAME_SEPARATOR)[0]
		}

		controller := controllers[controllerName]
		if controller == nil {
			Help(bot, update, []string{})
			return
		}

		controller(bot, update, parts[1:])
	}
}

func worker(bot tgbotapi.BotAPI) {

}

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		go routeCommand(bot, update)
	}
}
