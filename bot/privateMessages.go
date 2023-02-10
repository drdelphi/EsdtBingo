package bot

import (
	"fmt"

	"github.com/DrDelphi/EsdtBingoBot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func (b *Bot) privateMessageReceived(message *tgbotapi.Message) {
	user := b.getOrCreateUser(message.From)
	name := utils.FormatTgUser(message.From)
	log.Info("private message received", "message", message.Text, "user", name)

	switch message.Text {
	case menuAbout:
		msg := tgbotapi.NewMessage(user.ID, aboutMessage)
		msg.ParseMode = tgbotapi.ModeMarkdown
		b.tgBot.Send(msg)
		return
	case menuMainHelp:
		msg := tgbotapi.NewMessage(user.ID, helpMessage)
		msg.ParseMode = tgbotapi.ModeMarkdown
		_, err := b.tgBot.Send(msg)
		if err != nil {
			log.Error("unable to send message", "message", helpMessage, "error", err)
		}
		return
	case menuGameInfo:
		b.sendGameInfo(user)
		return
	case menuBalance:
		balance, err := b.networkManager.GetBalance(user.Wallet)
		if err != nil {
			b.reportError("can not get wallet balance")
			return
		}
		text := fmt.Sprintf("`Wallet:` [%s](%s%s)\n`Balance:` %s eGLD",
			utils.ShortenAddress(user.Wallet), b.cfg.Network.ExplorerAccount, user.Wallet, utils.NicePrice(balance, -1))
		if contractInfo.TickerIdentifier != "" {
			tokenBalance, _ := b.networkManager.GetTokenBalance(user.Wallet, contractInfo.TickerIdentifier)
			text += fmt.Sprintf("\n`Token Balance:` %s %s", utils.NicePrice(tokenBalance, -1), contractInfo.TickerIdentifier)
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔑 PEM file", "PEM"),
		))
		msg := tgbotapi.NewMessage(user.ID, text)
		msg.ReplyMarkup = keyboard
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.DisableWebPagePreview = true
		b.tgBot.Send(msg)
		return
	case menuBuyTicket:
		b.buyTicket(user, 1)
		return
	case menuBuy2:
		b.buyTicket(user, 2)
		return
	case menuBuy3:
		b.buyTicket(user, 3)
		return
	case menuBuy4:
		b.buyTicket(user, 4)
		return
	case menuBuy5:
		b.buyTicket(user, 5)
		return
	case menuBuy6:
		b.buyTicket(user, 6)
		return
	case menuMyTickets:
		b.sendMyTickets(user)
		return
	case menuStatistics:
		b.sendStatistics(user)
		return
	}
}
