module github.com/joncooperworks/cryptopricesbot

go 1.14

// +heroku install ./bot
require (
	github.com/hunterlong/shapeshift v0.0.0-20180821065152-75f73203a884
	github.com/joncooperworks/go-coinmarketcap v0.0.0-20171113052712-0b5c6df9fd62
	github.com/joncooperworks/jsonjse v0.0.0-20200525110930-7b1596308c3d
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
	gopkg.in/telegram-bot-api.v4 v4.6.4
)
