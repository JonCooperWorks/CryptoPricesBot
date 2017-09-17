package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"net/http"
	"os"
	"strings"
)

/* Web Services config */
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

/* Controller routing table */
var (
	controllers = map[string]Controller{
		START_COMMAND: Start,
		QUOTE_COMMAND: Quote,
		HELP_COMMAND:  Help,
	}
)

/* Currencies returned in coincap.io responses */
var (
	CURRENCIES = map[string]string{
		"USD": "price_usd",
		"EUR": "price_eur",
		"LTC": "price_ltc",
		"BTC": "price_btc",
		"ETH": "price_eth",
		"ZEC": "price_zec",
	}

	SYMBOLS = map[string]string{
		"USD": "$",
		"EUR": "€",
		"LTC": " Ł",
		"ETH": "Ξ",
		"BTC": "₿",
		"ZEC": "ZEC",
	}
)

/* Messages */
var (
	WELCOME_MESSAGE string = "Ask me for prices with /quote (ticker). Example: /quote BTC"
	HELP_MESSAGE    string = "Use me to get prices from https://coincap.io. Just type /quote (Ticker Symbol). For " +
		"example, /quote BTC or /quote BTC EUR"
)

type Controller func(*tgbotapi.BotAPI, tgbotapi.Update, []string)
type Command struct {
	Controller Controller
	Arguments  []string
}

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
	var comparisonCurrency string
	if len(arguments) == 2 {
		comparisonCurrency = strings.ToUpper(arguments[1])
	} else {
		comparisonCurrency = "USD"
	}

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

	quoteMessage := getQuoteFormat(comparisonCurrency, ticker, coinQuoteResponse)
	reply(bot, update, quoteMessage)

}

func getQuoteFormat(comparisonCurrency string, ticker string, coinQuoteResponse map[string]interface{}) string {
	propertyName := CURRENCIES[comparisonCurrency]
	if propertyName == "" {
		propertyName = "price_usd"
	}
	coinPrice := coinQuoteResponse[propertyName]
	var quoteMessage string
	switch coinPrice.(type) {
	case float64, float32:
		if coinPrice.(float64) < 0.99 {
			quoteMessage = "1 %s = %s%s%.8f"
		} else {
			quoteMessage = "1 %s = %s%.2f"
		}
	}

	symbol := SYMBOLS[comparisonCurrency]
	return fmt.Sprintf(quoteMessage, ticker, symbol, coinPrice)
}

func reply(bot *tgbotapi.BotAPI, update tgbotapi.Update, message string) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
	msg.ReplyToMessageID = update.Message.MessageID
	bot.Send(msg)
}

func routeCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	command, err := parseCommandFromUpdate(update)
	if err != nil {
		Help(bot, update, []string{})
		return
	}
	command.Controller(bot, update, command.Arguments)
}

func parseCommandFromUpdate(update tgbotapi.Update) (*Command, error) {
	if !update.Message.IsCommand() {
		arguments := parseArgumentsFromUpdate(update.Message.Text)
		return &Command{
			Controller: Quote,
			Arguments:  arguments,
		}, nil
	}

	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
	parts := parseArgumentsFromUpdate(update.Message.Text)
	if len(parts) < 1 {
		return nil, errors.New(HELP_MESSAGE)
	}

	controllerName := parts[0]
	if strings.Contains(controllerName, BOT_NAME) {
		controllerName = strings.Split(controllerName, USERNAME_SEPARATOR)[0]
	}

	controller := controllers[controllerName]
	if controller == nil {
		return nil, errors.New(HELP_MESSAGE)
	}

	return &Command{
		Controller: controller,
		Arguments:  parts[1:],
	}, nil
}
func parseArgumentsFromUpdate(message string) []string {
	return strings.Split(message, " ")
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
