package bot

import (
	"github.com/DrDelphi/EsdtBingoBot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func (b *Bot) privateCommandReceived(message *tgbotapi.Message) {
	cmd := message.Command()
	args := message.CommandArguments()
	name := utils.FormatTgUser(message.From)

	user := b.getOrCreateUser(message.From)
	log.Info("private command received", "command", cmd, "args", args, "user", name)

	if cmd == "start" {
		msg := tgbotapi.NewMessage(user.ID, helpMessage)
		msg.ParseMode = tgbotapi.ModeMarkdown
		b.tgBot.Send(msg)
		b.mainMenu(user)
		return
	}
}
