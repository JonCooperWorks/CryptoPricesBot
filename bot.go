package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/hunterlong/shapeshift"
	"gopkg.in/telegram-bot-api.v4"
	"runtime"
	"strconv"
	"math"
)

/* Web Services config */
const (
	PRICE_API_ENDPOINT = "https://coincap.io/page/%s"
	USERNAME_SEPARATOR = "@"
	BOT_NAME           = USERNAME_SEPARATOR + "coincap_prices_bot"
)

/* Commands */
const (
	QUOTE_COMMAND   = "/quote"
	START_COMMAND   = "/start"
	HELP_COMMAND    = "/help"
	CONVERT_COMMAND = "/convert"
)

/* Controller routing table */
var (
	controllers = map[string]Controller{
		START_COMMAND:   StartCommand,
		QUOTE_COMMAND:   QuoteCommand,
		HELP_COMMAND:    HelpCommand,
		CONVERT_COMMAND: ConvertCommand,
	}
)

/* Fiat currencies returned in coincap.io responses */
var (
	FIAT_CURRENCIES = map[string]string{
		"USD": "price_usd",
		"EUR": "price_eur",
	}

	SYMBOLS = map[string]string{
		"USD": "US$",
		"EUR": "€",
		"LTC": " Ł",
		"ETH": "Ξ",
		"BTC": "฿",
		"ZEC": "ZEC ",
	}
)

/* Messages */
const (
	WELCOME_MESSAGE = "Ask me for prices with /quote (ticker).\n" +
		"Example: /quote BTC or /quote BTC EUR.\n" +
		"You can also convert specific amounts with /convert (amount) (from) (to).\n" +
		"Example: /convert 100 BTC USD"
	HELP_MESSAGE = "Use me to get prices from https://coincap.io and https://shapeshift.io.\n" +
		"Just type /quote (First Symbol).\n" +
		"For example, /quote BTC or /quote BTC EUR.\n" +
		"To convert a specific amount, use the convert command.\n" +
		"For example, /convert 100 USD BTC.\n" +
		"Supported currencies: USD, EUR and all cryptocurrency pairs on https://shapeshift.io."
	COINCAP_BAD_RESPONSE_MESSAGE         = "I can't read the response from https://coincap.io for '%s.'"
	COINCAP_UNAVAILABLE_MESSAGE          = "I'm having trouble reaching https://coincap.io. Try again later."
	COIN_NOT_FOUND_ON_COINCAP_MESSAGE    = "I can't find '%s' on https://coincap.io"
	SHAPESHIFT_UNAVAILABLE_MESSAGE       = "I'm having trouble contacting https://shapeshift.io. Try again later."
	COIN_NOT_FOUND_ON_SHAPESHIFT_MESSAGE = "Error looking up %s/%s on https://shapeshift.io.\n%s"
)

type Controller func(*tgbotapi.BotAPI, tgbotapi.Update, []string)
type Command struct {
	Controller Controller
	Arguments  []string
}
type Quote struct {
	Second string
	First  string
	Price  float64
	Amount float64
}

func StartCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	reply(bot, update, WELCOME_MESSAGE)
}

func HelpCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	reply(bot, update, HELP_MESSAGE)
}

func ConvertCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	if len(arguments) != 3 {
		HelpCommand(bot, update, arguments)
		return
	}
	amount, err := strconv.ParseFloat(arguments[0], 64)
	if err != nil {
		log.Println(err.Error())
		HelpCommand(bot, update, arguments)
		return
	}
	first := strings.ToUpper(arguments[1])
	second := strings.ToUpper(arguments[2])
	quote, err := NewQuote(first, second, amount)
	if err != nil {
		reply(bot, update, err.Error())
		return
	}

	reply(bot, update, quote.String())
}

func QuoteCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	if len(arguments) < 1 {
		HelpCommand(bot, update, arguments)
		return
	}

	first := strings.ToUpper(arguments[0])
	var second string
	if len(arguments) == 2 {
		second = strings.ToUpper(arguments[1])
	} else {
		second = "USD"
	}

	quote, err := NewQuote(first, second, 1)
	if err != nil {
		reply(bot, update, err.Error())
		return
	}
	reply(bot, update, quote.String())
}

func isFiatInvolved(first, second string) bool {
	return isFiat(first) || isFiat(second)
}
func isFiat(ticker string) bool {
	return FIAT_CURRENCIES[ticker] != ""
}

func NewQuote(first, second string, amount float64) (*Quote, error) {
	log.Printf("Looking up %s/%s", first, second)
	var coinPrice float64
	if isFiatInvolved(first, second) {
		var url string
		if isFiat(first) {
			url = fmt.Sprintf(PRICE_API_ENDPOINT, second)
		} else {
			url = fmt.Sprintf(PRICE_API_ENDPOINT, first)
		}

		log.Printf("Looking up price at %s", url)
		response, err := http.Get(url)
		if err != nil {
			return nil, errors.New(COINCAP_UNAVAILABLE_MESSAGE)
		}

		if response.ContentLength == 2 {
			return nil, errors.New(fmt.Sprintf(COIN_NOT_FOUND_ON_COINCAP_MESSAGE, first))
		}

		var coinQuoteResponse map[string]interface{}
		err = json.NewDecoder(response.Body).Decode(&coinQuoteResponse)
		if err != nil {
			return nil, errors.New(fmt.Sprintf(COINCAP_BAD_RESPONSE_MESSAGE, first))
		}
		var rawCoinPrice interface{}
		if isFiat(first) {
			rawCoinPrice = coinQuoteResponse[FIAT_CURRENCIES[first]]
		} else {
			rawCoinPrice = coinQuoteResponse[FIAT_CURRENCIES[second]]
		}

		switch rawCoinPrice.(type) {
		case float64, float32:
			if isFiat(second) {
				coinPrice = rawCoinPrice.(float64)
			} else {
				// Fees and whatnot
				coinPrice = 0.993 / rawCoinPrice.(float64)
			}
		default:
			log.Printf("Coin price for %s/%s is not a float or numeric type, got: %v", first, second, rawCoinPrice)
			return nil, errors.New(
				fmt.Sprintf(COINCAP_BAD_RESPONSE_MESSAGE, first),
			)
		}
	} else {
		pair := shapeshift.Pair{Name: fmt.Sprintf("%s_%s", first, second)}
		log.Printf("Contacting https://shapeshift.io for %s", pair.Name)
		info, err := pair.GetInfo()
		if err != nil {
			return nil, errors.New(SHAPESHIFT_UNAVAILABLE_MESSAGE)
		}

		if info.ErrorMsg() != "" {
			log.Println(info.ErrorMsg())
			return nil, errors.New(fmt.Sprintf(COIN_NOT_FOUND_ON_SHAPESHIFT_MESSAGE, first, second, info.ErrorMsg()))
		}

		coinPrice = info.Rate
	}
	return &Quote{
		First:  first,
		Second: second,
		Price:  coinPrice,
		Amount: amount,
	}, nil
}

func (quote *Quote) String() string {
	var quoteMessage string
	cost := quote.Price * quote.Amount
	if quote.Amount < 1 {
		quoteMessage = "%.8f %s = "
	} else if math.Mod(quote.Amount, 1) == 0 {
		quoteMessage = "%.0f %s = "
	} else {
		quoteMessage = "%.2f %s = "
	}

	if cost < 1 {
		quoteMessage += "%s%.8f"
	} else {
		quoteMessage += "%s%.2f"
	}
	symbol := SYMBOLS[quote.Second]
	if symbol == "" {
		symbol = quote.Second
	}
	return fmt.Sprintf(quoteMessage, quote.Amount, quote.First, symbol, cost)
}

func reply(bot *tgbotapi.BotAPI, update tgbotapi.Update, message string) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
	msg.ReplyToMessageID = update.Message.MessageID
	bot.Send(msg)
}

func routeCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	command, err := NewCommand(update)
	if err != nil {
		log.Println(err.Error())
		HelpCommand(bot, update, []string{})
		return
	}
	command.Controller(bot, update, command.Arguments)
}

func NewCommand(update tgbotapi.Update) (*Command, error) {
	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
	parts := parseArgumentsFromUpdate(update.Message.Text)
	if len(parts) < 1 {
		return nil, errors.New(fmt.Sprintf("Error parsing arguments from %s", update.Message.Text))
	}

	if !update.Message.IsCommand() {
		return &Command{
			Controller: QuoteCommand,
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

func worker(updates <-chan tgbotapi.Update, bot *tgbotapi.BotAPI) {
	for update := range updates {
		if update.Message == nil {
			continue
		}

		routeCommand(bot, update)
	}
}

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = os.Getenv("DEBUG") != ""

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
	}

	// Goroutine pool for processing messages.
	for i := 0; i < runtime.NumCPU()-1; i++ {
		go worker(updates, bot)
	}
	// Block on the main thread so we don't exit
	worker(updates, bot)
}
