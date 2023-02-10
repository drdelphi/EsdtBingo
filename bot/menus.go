package bot

import (
	"github.com/DrDelphi/EsdtBingoBot/data"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func (b *Bot) mainMenu(user *data.User) {
	menu := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(menuGameInfo),
			tgbotapi.NewKeyboardButton(menuStatistics),
			tgbotapi.NewKeyboardButton(menuBalance),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(menuBuyTicket),
			tgbotapi.NewKeyboardButton(menuMyTickets),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(menuBuy2),
			tgbotapi.NewKeyboardButton(menuBuy3),
			tgbotapi.NewKeyboardButton(menuBuy4),
			tgbotapi.NewKeyboardButton(menuBuy5),
			tgbotapi.NewKeyboardButton(menuBuy6),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(menuMainHelp),
			tgbotapi.NewKeyboardButton(menuAbout),
		),
	)

	msg := tgbotapi.NewMessage(user.ID, "`🏘 Main menu`")
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = menu
	b.tgBot.Send(msg)
}
