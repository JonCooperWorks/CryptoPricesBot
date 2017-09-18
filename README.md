CoinCapBot
===========
CoinCapBot is a Telegram bot meant to get the prices of cryptocurrencies from https://coincap.io
and https://shapeshift.io.

To run an instance on your machine, get a Telegram Bot API key and start the bot using the following command:

```
go get github.com/go-telegram-bot-api/telegram-bot-api
go get github.com/hunterlong/shapeshift
TELEGRAM_BOT_TOKEN=(your token) go run bot.go
```

Or build with ```go build```