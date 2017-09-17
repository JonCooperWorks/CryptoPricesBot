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
		"USD": "US$",
		"EUR": "€",
		"LTC": " Ł",
		"ETH": "Ξ",
		"BTC": "฿",
		"ZEC": "ZEC",
	}
)

/* Messages */
const (
	WELCOME_MESSAGE string = "Ask me for prices with /quote (ticker). Example: /quote BTC or /quote BTC EUR"
	HELP_MESSAGE    string = "Use me to get prices from https://coincap.io. Just type /quote (Ticker Symbol).\n" +
		"For example, /quote BTC or /quote BTC EUR.\n" +
		"Supported currencies: USD, EUR, BTC, LTC, ETH, ZEC"
	COINCAP_BAD_RESPONSE_MESSAGE = "I can't read the response from https://coincap.io for '%s'"
	COIN_NOT_FOUND_MESSAGE = "I can't find '%s' on https://coincap.io"
	COINCAP_UNAVAILABLE_MESSAGE = "I'm having trouble reaching https://coincap.io. Try again later."
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
		reply(bot, update, COINCAP_UNAVAILABLE_MESSAGE)
		return
	}

	if response.ContentLength == 2 {
		reply(bot, update, fmt.Sprintf(COIN_NOT_FOUND_MESSAGE, ticker))
		return
	}
	var coinQuoteResponse map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&coinQuoteResponse)
	if err != nil {
		log.Println(err.Error())
		reply(bot, update, fmt.Sprintf(COINCAP_BAD_RESPONSE_MESSAGE, ticker))
		return
	}

	quoteMessage, err := getQuoteFormat(comparisonCurrency, ticker, coinQuoteResponse)
	if err != nil {
		log.Println(err.Error())
		Help(bot, update, []string{})
	}
	reply(bot, update, quoteMessage)

}

func getQuoteFormat(comparisonCurrency string, ticker string, coinQuoteResponse map[string]interface{}) (string, error) {
	propertyName := CURRENCIES[comparisonCurrency]
	if propertyName == "" {
		return "", errors.New("Invalid currency passed: " + comparisonCurrency)
	}
	coinPrice := coinQuoteResponse[propertyName]
	var quoteMessage string
	switch coinPrice.(type) {
	case float64, float32:
		if coinPrice.(float64) < 1.00 {
			quoteMessage = "1 %s = %s%.8f"
		} else {
			quoteMessage = "1 %s = %s%.2f"
		}
	}

	symbol := SYMBOLS[comparisonCurrency]
	if symbol == "" {
		symbol = comparisonCurrency
	}
	return fmt.Sprintf(quoteMessage, ticker, symbol, coinPrice), nil
}

func reply(bot *tgbotapi.BotAPI, update tgbotapi.Update, message string) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
	msg.ReplyToMessageID = update.Message.MessageID
	bot.Send(msg)
}

func routeCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	command, err := parseCommandFromUpdate(update)
	if err != nil {
		log.Println(err.Error())
		Help(bot, update, []string{})
		return
	}
	command.Controller(bot, update, command.Arguments)
}

func parseCommandFromUpdate(update tgbotapi.Update) (*Command, error) {
	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
	parts := parseArgumentsFromUpdate(update.Message.Text)
	if len(parts) < 1 {
		return nil, errors.New(fmt.Sprintf("Error parsing arguments from %s", update.Message.Text))
	}

	if !update.Message.IsCommand() {
		return &Command{
			Controller: Quote,
			Arguments:  parts,
		}, nil
	}

	controllerName := parts[0]
	if strings.Contains(controllerName, BOT_NAME) {
		controllerName = strings.Split(controllerName, USERNAME_SEPARATOR)[0]
	}

	controller := controllers[controllerName]
	if controller == nil {
		return nil, errors.New(fmt.Sprintf("Controller '%s' not found", controllerName))
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

	bot.Debug = os.Getenv("DEBUG") != ""

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
