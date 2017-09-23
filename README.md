CoinCapBot
===========
CoinCapBot is a Telegram bot meant to get the prices of cryptocurrencies from https://coincap.io
and https://shapeshift.io.

To run an instance on your machine, get a Telegram Bot API key and start the bot using the following command:

```
go get -t github.com/hunterlong/shapeshift
go get -t github.com/patrickmn/go-cache
go get -t gopkg.in/telegram-bot-api.v4
go get -t github.com/PuerkitoBio/goquery

TELEGRAM_BOT_TOKEN=(your token) go run bot.go
```

Or build with ```go build```