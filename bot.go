package main

import (
	"log"
	"gopkg.in/telegram-bot-api.v4"
	"net/http"
	"fmt"
	"encoding/json"
	"strings"
	"os"
)

const (
	PRICE_API_ENDPOINT = "https://coincap.io/page/%s"
)

var (
	commands = map[string]func(*tgbotapi.BotAPI, tgbotapi.Update, []string){
		"/start": Start,
		"/quote": Quote,
	}
)

func Start(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	reply(bot, update, "Ask me for prices with /quote (ticker). Example: /quote BTC")
}

func Quote(bot *tgbotapi.BotAPI, update tgbotapi.Update, arguments []string) {
	if len(arguments) < 1 {
		reply(bot, update, "Usage: /quote (ticker)")
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
	}
	var coinQuoteResponse map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&coinQuoteResponse)
	if err != nil {
		log.Println(err.Error())
		reply(bot, update, fmt.Sprintf("Error decoding response for %s", ticker))
		return
	}
	quoteMessage := fmt.Sprintf("1 %s = USD$%.2f", ticker, coinQuoteResponse["price_usd"])
	log.Println(quoteMessage)
	reply(bot, update, quoteMessage)

}

func reply(bot *tgbotapi.BotAPI, update tgbotapi.Update, message string) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
	msg.ReplyToMessageID = update.Message.MessageID
	bot.Send(msg)
}

func routeMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.Message.GroupChatCreated {
		return
	}

	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
	parts := strings.Split(update.Message.Text, " ")
	if len(parts) < 1 {
		reply(bot, update, "Invalid command")
		return
	}
	controller := commands[parts[0]]
	if controller == nil {
		reply(bot, update, "Command not found")
		return
	}

	controller(bot, update, parts[1:])
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
		routeMessage(bot, update)
	}
}