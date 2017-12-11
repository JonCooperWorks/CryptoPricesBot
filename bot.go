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
	"time"

	"reflect"

	"github.com/PuerkitoBio/goquery"
	"github.com/hunterlong/shapeshift"
	coinApi "github.com/joncooperworks/go-coinmarketcap"
	"github.com/patrickmn/go-cache"
	"gopkg.in/telegram-bot-api.v4"
)

/* Web Services config */
const (
	CEX_IO_PRICE_API_ENDPOINT   = "https://cex.io/api/ticker/%s/%s"
	JSE_PRICE_SCRAPING_ENDPOINT = "https://www.jamstockex.com"
	USERNAME_SEPARATOR          = "@"
	BOT_NAME                    = USERNAME_SEPARATOR + "coincap_prices_bot"
)

/* Commands */
const (
	QUOTE_COMMAND                 = "/quote"
	START_COMMAND                 = "/start"
	HELP_COMMAND                  = "/help"
	CONVERT_COMMAND               = "/convert"
	SOURCE_COMMAND                = "/source"
	JSE_QUOTE_COMMAND             = "/wahgwaanfi"
	ALTERNATIVE_JSE_QUOTE_COMMAND = "/wagwaanfi"
	CEX_IO_COMMAND                = "/cexprice"
	TRADE_COMMAND                 = "/trade"
)

/* Controller routing table */
var (
	controllers = map[string]Controller{
		START_COMMAND:                 StartCommand,
		QUOTE_COMMAND:                 QuoteCommand,
		HELP_COMMAND:                  HelpCommand,
		CONVERT_COMMAND:               ConvertCommand,
		SOURCE_COMMAND:                SourceCommand,
		JSE_QUOTE_COMMAND:             JseQuoteCommand,
		ALTERNATIVE_JSE_QUOTE_COMMAND: JseQuoteCommand,
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
	WELCOME_MESSAGE = "Ask me for prices with /quote (ticker).\n" +
		"Example: /quote BTC or /quote BTC EUR.\n\n" +
		"You can also convert specific amounts with /convert (amount) (from) (to).\n" +
		"Example: /convert 100 BTC USD.\n\n" +
		"I can also tell you wah gwaan fi a stock on the Jamaica Stock Exchange " +
		"(http://jamstockex.com/market-data/combined-market/summary/)\n" +
		"Example: /wahgwaanfi NCBFG"
	HELP_MESSAGE = "Use me to get prices from https://coinmarketcap.com and https://shapeshift.io.\n" +
		"Just type /quote (First Symbol).\n" +
		"For example, /quote BTC or /quote BTC EUR.\n" +
		"To convert a specific amount, use the convert command.\n" +
		"For example, /convert 100 USD BTC.\n" +
		"Supported currencies for cryptocurrency lookups: USD, EUR and all cryptocurrency pairs on https://shapeshift.io.\n" +
		"I can also tell you wah gwaan fi stocks on the Jamaica Stock Exchange.\n" +
		"For example, /wahgwaanfi NCBFG"
	SHAPESHIFT_UNAVAILABLE_MESSAGE       = "I'm having trouble contacting https://shapeshift.io. Try again later."
	COIN_NOT_FOUND_ON_SHAPESHIFT_MESSAGE = "Error looking up '%s/%s' on https://shapeshift.io.\n%s"
	CONVERT_AMOUNT_NUMERIC_MESSAGE       = "Only numbers can be used with /convert.\n" +
		"Do not use a currency symbol."
	SOURCE_MESSAGE = "You can find my source code here: " +
		"https://github.com/JonCooperWorks/CryptoPricesBot.\n" +
		"My code is licensed GPLv3, so you're free to use and modify it if you open source your modifications."
)

/* JSE Messages */
const (
	JSE_UNAVAILABLE_MESSAGE = "I can't read the response from the JSE website.\n" +
		"Try again later or ask me about a cryptocurrency.\n" +
		"https://twitter.com/jastockex?lang=en"
	JSE_STOCK_NOT_FOUND_MESSAGE = "I can't find '%s' on the JSE."
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
	SHAPESHIFT_SOURCE_URL = "https://shapeshift.io/#/coins"
	COINCAP_SOURCE_URL    = "https://coinmarketcap.com"
	JSE_SOURCE_URL        = "https://www.jamstockex.com/ticker-data"
)

/* JSE Cache */
var (
	JSE_CACHE = cache.New(10*time.Minute, 10*time.Minute)
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

	quoteMessage += "."

	if quote.SourceUrl != "" {
		quoteMessage += "\n\nQuote fetched from: %s"
	} else {
		quoteMessage += "%s"
	}

	symbol := SYMBOLS[quote.Second]
	if symbol == "" {
		symbol = quote.Second
	}
	return fmt.Sprintf(quoteMessage, quote.Amount, quote.First, symbol, cost, quote.SourceUrl)
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

func ConvertCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	if len(arguments) != 3 {
		HelpCommand(bot, update, arguments)
		return
	}
	amount, err := strconv.ParseFloat(arguments[0], 64)
	if err != nil {
		log.Println(err.Error())
		reply(bot, update, CONVERT_AMOUNT_NUMERIC_MESSAGE)
		return
	}
	first := strings.ToUpper(arguments[1])
	second := strings.ToUpper(arguments[2])
	quote, err := NewCryptoQuote(first, second, amount)
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

	quote, err := NewCryptoQuote(first, second, 1)
	if err != nil {
		reply(bot, update, err.Error())
		return
	}
	reply(bot, update, quote.String())
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
	rawPrice, found := JSE_CACHE.Get(ticker)
	if !found {
		resp, err := http.Get(JSE_PRICE_SCRAPING_ENDPOINT)
		log.Printf("Looking up %s on the JSE at %s", ticker, JSE_PRICE_SCRAPING_ENDPOINT)
		if err != nil {
			log.Println(err.Error())
			return 0, errors.New(JSE_UNAVAILABLE_MESSAGE)
		}

		if resp.StatusCode != 200 {
			log.Println("Got status ", resp.StatusCode, " from the JSE looking up ", ticker)
			return 0, errors.New(JSE_UNAVAILABLE_MESSAGE)
		}
		document, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			log.Println(err.Error())
			return 0, errors.New(JSE_UNAVAILABLE_MESSAGE)
		}

		document.Find("li").Each(
			func(i int, s *goquery.Selection) {
				stockInformation := s.Find("a").Text()
				stockQuote := []string{}
				for _, datapoint := range strings.Split(stockInformation, "\n") {
					datapoint = strings.TrimSpace(datapoint)
					if strings.TrimSpace(datapoint) == "" {
						continue
					}
					stockQuote = append(stockQuote, datapoint)
				}
				// Skip those random empty entries
				if len(stockQuote) < 4 {
					return
				}

				ticker := stockQuote[0]
				// Get the "$" out of the price
				var index int
				if len(stockQuote) == 4 {
					index = 2
				} else {
					index = 3
				}
				rawPrice := strings.Replace(stockQuote[index][1:], ",", "", -1)
				price, err := strconv.ParseFloat(rawPrice, 64)
				if err != nil {
					log.Println(err.Error())
					return
				}
				JSE_CACHE.Add(ticker, price, cache.DefaultExpiration)
			})

		rawPrice, found = JSE_CACHE.Get(ticker)
		if !found {
			return 0, errors.New(fmt.Sprintf(JSE_STOCK_NOT_FOUND_MESSAGE, ticker))
		}
	}
	return rawPrice.(float64), nil
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

func NewCryptoQuote(first, second string, amount float64) (*Quote, error) {
	if isFiatInvolved(first, second) {
		return NewCoinMarketCapQuote(first, second, amount)
	} else {
		return NewShapeShiftQuote(first, second, amount)
	}
}

func NewShapeShiftQuote(first, second string, amount float64) (*Quote, error) {
	log.Printf("Looking up %s/%s", first, second)
	pair := shapeshift.Pair{Name: fmt.Sprintf("%s_%s", first, second)}
	log.Printf("Contacting https://shapeshift.io for '%s'", pair.Name)
	info, err := pair.GetInfo()
	if err != nil {
		return nil, errors.New(SHAPESHIFT_UNAVAILABLE_MESSAGE)
	}

	if info.ErrorMsg() != "" {
		log.Println(info.ErrorMsg())
		return nil, errors.New(fmt.Sprintf(COIN_NOT_FOUND_ON_SHAPESHIFT_MESSAGE, first, second, info.ErrorMsg()))
	}

	return &Quote{
		First:     first,
		Second:    second,
		Price:     info.Rate,
		Amount:    amount,
		SourceUrl: SHAPESHIFT_SOURCE_URL,
	}, nil
}

func NewCoinMarketCapQuote(first, second string, amount float64) (*Quote, error) {
	var ticker string
	var base string
	if isFiat(first) {
		base = first
		ticker = second
	} else {
		base = second
		ticker = first
	}
	coinInfo, err := coinApi.GetAllCoinData(base)
	if err != nil {
		log.Println(err.Error())
		return nil, errors.New(fmt.Sprintf("Could not find %s/%s on https://coinmarketcap.com", first, second))
	}

	var coinPrice float64
	found := false
	for _, coin := range coinInfo {
		if coin.Symbol == ticker {
			r := reflect.ValueOf(coin)
			fieldName := FIAT_CURRENCIES[base]
			log.Printf("Getting conversion for %s at coin.%s", base, fieldName)
			rawCoinPrice := reflect.Indirect(r).FieldByName(fieldName)
			coinPrice = rawCoinPrice.Float()
			found = true
			break
		}
	}

	if !found {
		return nil, errors.New(fmt.Sprintf("%s/%s not found on https://coinmarketcap.com", first, second))
	}
	if isFiat(first) {
		coinPrice = 1 / coinPrice
	}

	return &Quote{
		First:     first,
		Second:    second,
		Price:     coinPrice,
		Amount:    amount,
		SourceUrl: COINCAP_SOURCE_URL,
	}, nil

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
			Controller: QuoteCommand,
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
