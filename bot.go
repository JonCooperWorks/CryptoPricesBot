package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/joncooperworks/jsonjse"
	"gopkg.in/telegram-bot-api.v4"
)

/* Web Services config */
const (
	CEX_IO_PRICE_API_ENDPOINT = "https://cex.io/api/ticker/%s/%s"
	USERNAME_SEPARATOR        = "@"
	BOT_NAME                  = USERNAME_SEPARATOR + "coincap_prices_bot"
)

/* Commands */
const (
	START_COMMAND                 = "/start"
	HELP_COMMAND                  = "/help"
	SOURCE_COMMAND                = "/source"
	JSE_QUOTE_COMMAND             = "/wahgwaanfi"
	ALTERNATIVE_JSE_QUOTE_COMMAND = "/wagwaanfi"
	YET_ANOTHER_JSE_COMMAND       = "/jse"
	CEX_IO_COMMAND                = "/cexprice"
)

/* Controller routing table */
var (
	controllers = map[string]Controller{
		START_COMMAND:                 StartCommand,
		HELP_COMMAND:                  HelpCommand,
		SOURCE_COMMAND:                SourceCommand,
		JSE_QUOTE_COMMAND:             JseQuoteCommand,
		ALTERNATIVE_JSE_QUOTE_COMMAND: JseQuoteCommand,
		YET_ANOTHER_JSE_COMMAND:       JseQuoteCommand,
		CEX_IO_COMMAND:                CexPriceCommand,
	}
)

/* Fiat currencies returned in coinmarketcap.com responses */
var (
	FIAT_CURRENCIES = map[string]string{
		"USD": "PriceUsd",
		"EUR": "PriceEur",
		"GBP": "PriceGbp",
	}

	SYMBOLS = map[string]string{
		"USD":   "US$",
		"EUR":   "€",
		"LTC":   "Ł",
		"ETH":   "Ξ",
		"BTC":   "฿",
		"XRP":   "Ʀ",
		"XMR":   "ɱ",
		"ETC":   "ξ",
		"REP":   "Ɍ",
		"STEEM": "ȿ",
		"DOGE":  "Ð",
		"ZEC":   "ⓩ",
		"JMD":   "J$",
		"GBP":   "£",
	}
)

/* Crypto Messages */
const (
	WELCOME_MESSAGE = "Ask me for JSE stock prices." +
		"Just send me the symbol. For example: NCBFG.\n" +
		"I can also tell you wah gwaan fi a stock on the Jamaica Stock Exchange " +
		"Example: /wahgwaanfi NCBFG"
	HELP_MESSAGE = "Use me to get prices from the Jamaica Stock Exchange.\n" +
		"Just send me the symbol. For example: NCBFG.\n" +
		"I can also tell you wah gwaan fi stocks on the Jamaica Stock Exchange.\n" +
		"For example, /wahgwaanfi NCBFG"
	SOURCE_MESSAGE = "You can find my source code here: " +
		"https://github.com/JonCooperWorks/CryptoPricesBot.\n" +
		"My code is licensed GPLv3, so you're free to use and modify it if you open source your modifications."
)

/* cex.io Messages */
const (
	CEX_IO_UNAVAILABLE_MESSAGE = "I can't reach https://cex.io right now.\n" +
		"Try again later."
	CEX_IO_BAD_RESPONSE_MESSAGE   = "I'm having trouble reading the response for '%s/%s' from https://cex.io."
	CEX_IO_PAIR_NOT_FOUND_MESSAGE = "I can't find '%s/%s' on https://cex.io"
)

/* Source URLs */
const (
	CEX_IO_SOURCE_URL     = "https://cex.io/r/0/up100029857/0/"
	JSE_SOURCE_URL        = "https://jsonjse.herokuapp.com/jse/today"
)

type Controller func(*tgbotapi.BotAPI, tgbotapi.Update, []string)

type Command struct {
	Controller Controller
	Arguments  []string
}

type Quote struct {
	Second    string
	First     string
	Price     float64
	Amount    float64
	SourceUrl string
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

	quoteMessage += ".\n\n"
	quoteMessage += "Shop at Afrodite for all your beauty needs. Unleash your inner goddess at https://www.afroditeja.com"

	symbol := SYMBOLS[quote.Second]
	if symbol == "" {
		symbol = quote.Second
	}
	return fmt.Sprintf(quoteMessage, quote.Amount, quote.First, symbol, cost)
}

func StartCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	reply(bot, update, WELCOME_MESSAGE)
}

func HelpCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	reply(bot, update, HELP_MESSAGE)
}

func SourceCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	reply(bot, update, SOURCE_MESSAGE)
}

func JseQuoteCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	if len(arguments) < 1 {
		HelpCommand(bot, update, arguments)
		return
	}

	// JMD only for now.
	first := strings.ToUpper(arguments[0])
	var second = "JMD"
	quote, err := NewJseQuote(first, second, 1)
	if err != nil {
		reply(bot, update, err.Error())
		return
	}

	reply(bot, update, quote.String())

}

func CexPriceCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
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

	quote, err := NewCexIoQuote(first, second, 1)
	if err != nil {
		reply(bot, update, err.Error())
		return
	}
	reply(bot, update, quote.String())
}

func NewCexIoQuote(first, second string, amount float64) (*Quote, error) {
	url := fmt.Sprintf(CEX_IO_PRICE_API_ENDPOINT, first, second)
	log.Printf("Looking up %s/%s at %s", first, second, url)
	resp, err := http.Get(url)
	log.Printf("Looking up '%s/%s' on cex.io", first, second)
	if err != nil || resp.StatusCode != 200 {
		log.Println("Cex.io unavailable.")
		return nil, errors.New(CEX_IO_UNAVAILABLE_MESSAGE)
	}

	var coinQuoteResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&coinQuoteResponse)
	if err != nil {
		log.Println(err)
		return nil, errors.New(fmt.Sprintf(CEX_IO_BAD_RESPONSE_MESSAGE, first, second))
	}

	rawCoinPrice, found := coinQuoteResponse["last"]
	if !found {
		return nil, errors.New(fmt.Sprintf(CEX_IO_PAIR_NOT_FOUND_MESSAGE, first, second))
	}
	coinPrice, err := strconv.ParseFloat(rawCoinPrice.(string), 64)
	if err != nil {
		log.Printf("Coin price for %s/%s is not a float or numeric type, got: '%v'", first, second, rawCoinPrice)
		return nil, errors.New(
			fmt.Sprintf(CEX_IO_BAD_RESPONSE_MESSAGE, first, second),
		)
	}

	return &Quote{
		First:     first,
		Second:    second,
		Price:     coinPrice,
		Amount:    amount,
		SourceUrl: CEX_IO_SOURCE_URL,
	}, nil
}

func getJsePrice(ticker string) (float64, error) {
	resp, err := http.Get(JSE_SOURCE_URL)
	if err != nil {
		return 0, err
	}

	var symbols []jsonjse.Symbol
	err = json.NewDecoder(resp.Body).Decode(&symbols)
	if err != nil {
		return 0, err
	}

	for _, symbol := range symbols {
		if symbol.Symbol == ticker {
			return symbol.LastTraded, nil
		}
	}
	return float64(0), fmt.Errorf("Could not find %v on the JSE", ticker)
}

func NewJseQuote(first, second string, amount float64) (*Quote, error) {
	// Return prices from cache
	price, err := getJsePrice(first)
	if err != nil {
		log.Printf("Could not find '%s' on the JSE", first)
		return nil, err
	}
	return &Quote{
		First:     first,
		Second:    second,
		Amount:    amount,
		Price:     price,
		SourceUrl: JSE_SOURCE_URL,
	}, nil
}

func isFiatInvolved(first, second string) bool {
	return isFiat(first) || isFiat(second)
}
func isFiat(ticker string) bool {
	return FIAT_CURRENCIES[ticker] != ""
}

func reply(bot *tgbotapi.BotAPI, update tgbotapi.Update, message string) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
	msg.ReplyToMessageID = update.Message.MessageID
	bot.Send(msg)
}

func routeCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	command, err := NewCommand(update)
	if command == nil {
		// STFU when there's no command
		return
	}
	if err != nil {
		log.Println(err.Error())
		HelpCommand(bot, update, []string{})
		return
	}
	command.Controller(bot, update, command.Arguments)
}

func NewCommand(update tgbotapi.Update) (*Command, error) {
	log.Printf("[%s - %s] %s", update.Message.From.UserName, update.Message.From.FirstName, update.Message.Text)
	parts := parseArgumentsFromUpdate(update.Message.Text)
	if len(parts) < 1 {
		return nil, errors.New(fmt.Sprintf("Error parsing arguments from '%s'", update.Message.Text))
	} else if len(parts) > 3 && !update.Message.IsCommand() {
		return nil, nil
	}

	if !update.Message.IsCommand() {
		return &Command{
			Controller: JseQuoteCommand,
			Arguments:  parts,
		}, nil
	}

	controllerName := strings.ToLower(parts[0])
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
		if update.Message == nil || strings.TrimSpace(update.Message.Text) == "" {
			continue
		}

		routeCommand(bot, update)
	}
}

func listenForWebhook(updates <-chan tgbotapi.Update, bot *tgbotapi.BotAPI) {
	err := http.ListenAndServe(":" + os.Getenv("PORT"), nil)
	if err != nil {
		log.Fatalf("Error starting webhook: %v", err)
	}
}

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = os.Getenv("DEBUG") != ""

	log.Printf("Authorized on account %s", bot.Self.UserName)

	_, err = bot.SetWebhook(tgbotapi.NewWebhook(os.Getenv("TELEGRAM_WEBHOOK_URL")))
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

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.ListenForWebhook("/")
	go listenForWebhook(updates, bot)

	// Goroutine pool for processing messages.
	for i := 0; i < runtime.NumCPU()-1; i++ {
		go worker(updates, bot)
	}
	// Block on the main thread so we don't exit
	worker(updates, bot)
}
