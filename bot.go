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

	"github.com/PuerkitoBio/goquery"
	"github.com/hunterlong/shapeshift"
	"github.com/patrickmn/go-cache"
	"gopkg.in/telegram-bot-api.v4"
)

/* Web Services config */
const (
	CRYPTO_PRICE_API_ENDPOINT   = "https://coincap.io/page/%s"
	JSE_PRICE_SCRAPING_ENDPOINT = "https://www.jamstockex.com/market-data/combined-market/summary/"
	USERNAME_SEPARATOR          = "@"
	BOT_NAME                    = USERNAME_SEPARATOR + "coincap_prices_bot"
)

/* Commands */
const (
	QUOTE_COMMAND     = "/quote"
	START_COMMAND     = "/start"
	HELP_COMMAND      = "/help"
	CONVERT_COMMAND   = "/convert"
	SOURCE_COMMAND    = "/source"
	JSE_QUOTE_COMMAND = "/wahgwaanfi"
	ALTERNATIVE_SPELLING_JSE_QUOTE_COMMAND = "/wagwaanfi"
)

/* Controller routing table */
var (
	controllers = map[string]Controller{
		START_COMMAND:     StartCommand,
		QUOTE_COMMAND:     QuoteCommand,
		HELP_COMMAND:      HelpCommand,
		CONVERT_COMMAND:   ConvertCommand,
		SOURCE_COMMAND:    SourceCommand,
		JSE_QUOTE_COMMAND: JseQuoteCommand,
		ALTERNATIVE_SPELLING_JSE_QUOTE_COMMAND: JseQuoteCommand,
	}
)

/* Fiat currencies returned in coincap.io responses */
var (
	FIAT_CURRENCIES = map[string]string{
		"USD": "price_usd",
		"EUR": "price_eur",
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
	}
)

/* Crypto Messages */
const (
	WELCOME_MESSAGE = "Ask me for prices with /quote (ticker).\n" +
		"Example: /quote BTC or /quote BTC EUR.\n" +
		"You can also convert specific amounts with /convert (amount) (from) (to).\n" +
		"Example: /convert 100 BTC USD.\n" +
		"I can also tell you wah gwaan fi a stock on the Jamaica Stock Exchange " +
		"(http://jamstockex.com/market-data/combined-market/summary/)\n" +
		"Example: /wahgwaanfi NCBFG"
	HELP_MESSAGE = "Use me to get prices from https://coincap.io and https://shapeshift.io.\n" +
		"Just type /quote (First Symbol).\n" +
		"For example, /quote BTC or /quote BTC EUR.\n" +
		"To convert a specific amount, use the convert command.\n" +
		"For example, /convert 100 USD BTC.\n" +
		"Supported currencies for cryptocurrency lookups: USD, EUR and all cryptocurrency pairs on https://shapeshift.io.\n" +
		"I can also tell you wah gwaan fi stocks on the Jamaica Stock Exchange.\n" +
		"For example, /wahgwaanfi NCBFG"
	COINCAP_BAD_RESPONSE_MESSAGE         = "I can't read the response from https://coincap.io for '%s/%s.'"
	COINCAP_UNAVAILABLE_MESSAGE          = "I'm having trouble reaching https://coincap.io. Try again later."
	COIN_NOT_FOUND_ON_COINCAP_MESSAGE    = "I can't find '%s/%s' on https://coincap.io."
	SHAPESHIFT_UNAVAILABLE_MESSAGE       = "I'm having trouble contacting https://shapeshift.io. Try again later."
	COIN_NOT_FOUND_ON_SHAPESHIFT_MESSAGE = "Error looking up %s/%s on https://shapeshift.io.\n%s"
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
	JSE_STOCK_NOT_FOUND_MESSAGE = "I can't find %s on the JSE."
)

/* JSE Cache */
var (
	// TODO: Expire next day at 2PM
	JSE_CACHE = cache.New(1*time.Hour, 2*time.Hour)
)

type Controller func(*tgbotapi.BotAPI, tgbotapi.Update, []string)
type QuoteSource func(first, second string, amount float64) (*Quote, error)

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

func scrapeJseWebsite(ticker string) (float64, error) {
	var price float64
	rawPrice, found := JSE_CACHE.Get(ticker)
	if !found {

		resp, err := http.Get(JSE_PRICE_SCRAPING_ENDPOINT)
		log.Printf("Looking up %s on the JSE", ticker)
		if err != nil || resp.StatusCode != 200 {
			log.Println("Patty dem run out.")
			return 0, errors.New(JSE_UNAVAILABLE_MESSAGE)
		}
		document, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			log.Println(err.Error())
			return 0, errors.New(JSE_UNAVAILABLE_MESSAGE)
		}

		// Get all the table rows and loop through them
		document.Find("table tbody tr").Each(func(i int, s *goquery.Selection) {
			// For each row, pull out the ticker and price
			uriContainingTicker, exists := s.Find("td a").Attr("href")
			if !exists {
				log.Println("No URL found")
				return
			}
			ticker := strings.TrimSpace(strings.Split(uriContainingTicker, "/")[4])
			ticker = strings.ToUpper(ticker)
			priceText := s.Find("td").Eq(2).Text()
			price, err := strconv.ParseFloat(strings.TrimSpace(priceText), 64)
			if err != nil {
				log.Println(err.Error())
				return
			}

			JSE_CACHE.Add(ticker, price, cache.DefaultExpiration)
		})

		price, err = scrapeJseWebsite(ticker)
		if err != nil {
			return 0, errors.New(fmt.Sprintf(JSE_STOCK_NOT_FOUND_MESSAGE, ticker))
		}
	} else {
		price = rawPrice.(float64)
	}
	return price, nil
}

func NewJseQuote(first, second string, amount float64) (*Quote, error) {
	// Return prices from cache
	var price interface{}
	var err error
	price, found := JSE_CACHE.Get(first)
	if !found {
		price, err = scrapeJseWebsite(first)
		if err != nil {
			return nil, err
		}
	}
	return &Quote{
		First:  first,
		Second: second,
		Amount: amount,
		Price:  price.(float64),
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
		return NewCoinCapQuote(first, second, amount)
	} else {
		return NewShapeShiftQuote(first, second, amount)
	}
}

func NewShapeShiftQuote(first, second string, amount float64) (*Quote, error) {
	log.Printf("Looking up %s/%s", first, second)
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

	return &Quote{
		First:  first,
		Second: second,
		Price:  info.Rate,
		Amount: amount,
	}, nil
}

func NewCoinCapQuote(first, second string, amount float64) (*Quote, error) {
	var url string
	if isFiat(first) {
		url = fmt.Sprintf(CRYPTO_PRICE_API_ENDPOINT, second)
	} else {
		url = fmt.Sprintf(CRYPTO_PRICE_API_ENDPOINT, first)
	}

	log.Printf("Looking up price at %s", url)
	response, err := http.Get(url)
	if err != nil {
		return nil, errors.New(COINCAP_UNAVAILABLE_MESSAGE)
	}

	if response.ContentLength == 2 {
		return nil, errors.New(fmt.Sprintf(COIN_NOT_FOUND_ON_COINCAP_MESSAGE, first, second))
	}

	var coinQuoteResponse map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&coinQuoteResponse)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(COINCAP_BAD_RESPONSE_MESSAGE, first, second))
	}
	var rawCoinPrice interface{}
	if isFiat(first) {
		rawCoinPrice = coinQuoteResponse[FIAT_CURRENCIES[first]]
	} else {
		rawCoinPrice = coinQuoteResponse[FIAT_CURRENCIES[second]]
	}

	var coinPrice float64
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
			fmt.Sprintf(COINCAP_BAD_RESPONSE_MESSAGE, first, second),
		)
	}

	return &Quote{
		First:  first,
		Second: second,
		Price:  coinPrice,
		Amount: amount,
	}, nil

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
	log.Printf("[%s - %s] %s", update.Message.From.UserName, update.Message.From.FirstName, update.Message.Text)
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
