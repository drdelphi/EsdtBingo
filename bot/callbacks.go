package bot

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/DrDelphi/EsdtBingoBot/data"
	"github.com/DrDelphi/EsdtBingoBot/utils"
	"github.com/ElrondNetwork/elrond-go-core/core/pubkeyConverter"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func (b *Bot) callbackQueryReceived(callback *tgbotapi.CallbackQuery) {
	cb := callback.Data
	b.tgBot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, cb))
	user := b.getOrCreateUser(callback.From)
	name := utils.FormatTgUser(callback.From)
	log.Info("callback received", "callback", callback.Data, "user", name)

	if callback.Data == "PEM" {
		b.sendPemFile(user)
		return
	}
}

func (b *Bot) sendPemFile(user *data.User) {
	pk := utils.GetPrivateKeyFromSeed(user.ID)
	address := utils.GetAddressFromPrivateKey(utils.GetPrivateKeyFromSeed(user.ID))
	converter, _ := pubkeyConverter.NewBech32PubkeyConverter(32, log)
	pubKey, _ := converter.Decode(address)
	seed := hex.EncodeToString(pk) + hex.EncodeToString(pubKey)
	b64 := base64.StdEncoding.EncodeToString([]byte(seed))

	filename := address + ".pem"
	f, err := os.Create(filename)
	if err != nil {
		return
	}

	fmt.Fprintf(f, "-----BEGIN PRIVATE KEY for %s-----\n", address)
	fmt.Fprintf(f, b64[:64]+"\n")
	fmt.Fprintf(f, b64[64:128]+"\n")
	fmt.Fprintf(f, b64[128:]+"\n")
	fmt.Fprintf(f, "-----END PRIVATE KEY for %s-----", address)
	f.Close()
	fileable := tgbotapi.NewDocumentUpload(user.ID, filename)
	b.tgBot.Send(fileable)
}
