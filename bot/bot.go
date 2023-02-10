package bot

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/DrDelphi/EsdtBingoBot/config"
	"github.com/DrDelphi/EsdtBingoBot/data"
	"github.com/DrDelphi/EsdtBingoBot/network"
	"github.com/DrDelphi/EsdtBingoBot/utils"
	logger "github.com/ElrondNetwork/elrond-go-logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var log = logger.GetOrCreate("bot")

var contractInfo *data.ContractInfo

// Bot - holds the required fields of the bot application
type Bot struct {
	tgBot          *tgbotapi.BotAPI
	cfg            *data.AppConfig
	networkManager *network.NetworkManager

	users   map[int64]*data.User
	tgUsers map[int64]*data.Telegram
}

// NewBot - creates a new Bot object
func NewBot(cfg *data.AppConfig, networkManager *network.NetworkManager) (*Bot, error) {
	tgBot, err := tgbotapi.NewBotAPI(cfg.Bot.Token)
	if err != nil {
		log.Error("can not create telegram bot", "error", err)
		return nil, err
	}

	telegramBot := &Bot{
		tgBot:          tgBot,
		cfg:            cfg,
		networkManager: networkManager,
		users:          make(map[int64]*data.User),
		tgUsers:        make(map[int64]*data.Telegram),
	}

	helpMessage = strings.ReplaceAll(helpMessage, "EsdtBingo", cfg.Bot.Group)

	return telegramBot, nil
}

// StartTasks - starts bot's tasks
func (b *Bot) StartTasks() {
	go func() {
		lastRound := uint64(0)
		lastBoughtTickets := uint64(0)
		lastInfoMessage := 0
		for {
			info, err := b.networkManager.GetContractInfo()
			if err != nil {
				b.reportError("Unable to get contract info. Error: " + err.Error())
			} else {
				contractInfo = info
				if contractInfo.GameRound != lastRound && contractInfo.Status != utils.StatusIdle {
					msg, err := b.sendGameInfo(nil)
					if err == nil {
						lastInfoMessage = msg.MessageID
					}
					lastRound = contractInfo.GameRound
					lastBoughtTickets = contractInfo.RoundTickets
				}
				if lastBoughtTickets != contractInfo.RoundTickets {
					msg := tgbotapi.NewEditMessageText(b.cfg.Bot.GroupID, lastInfoMessage, b.gameInfo(nil))
					msg.ParseMode = tgbotapi.ModeMarkdown
					b.tgBot.Send(msg)
				}
			}
			time.Sleep(time.Second * 6)
		}
	}()

	go func() {
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates, err := b.tgBot.GetUpdatesChan(u)
		if err != nil {
			log.Error("can not get Telegram bot updates", "error", err)
			panic(err)
		}
		updates.Clear()
		for update := range updates {
			if update.Message != nil {
				if update.Message.Chat.IsPrivate() {
					// private
					if update.Message.IsCommand() {
						b.privateCommandReceived(update.Message)
						continue
					}
					b.privateMessageReceived(update.Message)
				} else {
					// public
					if b.cfg.Bot.GroupID == 0 && update.Message.Chat.UserName == b.cfg.Bot.Group {
						b.cfg.Bot.GroupID = update.Message.Chat.ID
						_ = config.Save(b.cfg)
					}
					if update.Message.IsCommand() {
						b.tgBot.Send(tgbotapi.DeleteMessageConfig{ChatID: update.Message.Chat.ID, MessageID: update.Message.MessageID})
						continue
					}
				}
			}
			if update.CallbackQuery != nil {
				b.callbackQueryReceived(update.CallbackQuery)
			}
		}
	}()

	go func() {
		for {
			time.Sleep(time.Minute)
			if contractInfo != nil && contractInfo.Status == utils.StatusExtracting {
				hash, err := b.networkManager.SendTransaction(utils.ExtractNumbersPK, 0, utils.ExtractNumbersGasLimit, "extract_numbers", utils.AutoNonce)
				if err != nil {
					b.reportError("error sending extract numbers tx: " + err.Error())
				} else {
					lastNumbers, err := b.networkManager.GetLastExtractedNumbers()
					if err == nil {
						b.sendToGroup(fmt.Sprintf("`Extracted numbers:` `%v`\n", lastNumbers))
					}

					format := fmt.Sprintf("`Round #%v: Sending prizes` - Status: ", contractInfo.GameRound)
					msg := tgbotapi.NewMessage(b.cfg.Bot.GroupID, fmt.Sprintf("%s[pending ⌛️](%s%s)", format, b.cfg.Network.ExplorerTransaction, hash))
					msg.ParseMode = tgbotapi.ModeMarkdown
					msg.DisableWebPagePreview = true
					res, err := b.tgBot.Send(msg)
					if err != nil {
						log.Warn("can not send sending prizes tx status message", "message", msg.Text, "error", err)
					} else {
						b.watchExtractNumbersTx(hash, format, b.cfg.Bot.GroupID, res.MessageID)
					}
				}
			}
		}
	}()
}

func (b *Bot) reportError(text string) {
	msg := tgbotapi.NewMessage(b.cfg.Bot.Owner, "⛔️ "+text)
	b.tgBot.Send(msg)
}

func (b *Bot) sendToGroup(text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(b.cfg.Bot.GroupID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	res, err := b.tgBot.Send(msg)
	if err != nil {
		log.Warn("error sending message to group", "message", text, "error", err)
	}

	return res, err
}

func (b *Bot) sendMessage(userID int64, text string) (tgbotapi.Message, error) {
	user, ok := b.users[userID]
	if user == nil || !ok {
		return tgbotapi.Message{}, errors.New("user not found")
	}

	tgUser, ok := b.tgUsers[userID]
	if tgUser != nil && ok {
		log.Info("sent message", "user", fmt.Sprintf("@%s (%s %s)", tgUser.UserName, tgUser.FirstName, tgUser.LastName), "message", text)
	}
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	res, err := b.tgBot.Send(msg)
	if err != nil && ok {
		log.Warn("error sending message", "user", fmt.Sprintf("@%s (%s %s)", tgUser.UserName, tgUser.FirstName, tgUser.LastName),
			"message", text, "error", err.Error())
	}

	return res, err
}

func (b *Bot) gameInfo(user *data.User) string {
	if contractInfo == nil {
		return ""
	}

	text := "`Game Info`\n\n"
	text += fmt.Sprintf("`Game round:` #%v\n", contractInfo.GameRound)
	ticker := strings.Split(contractInfo.TickerIdentifier, "-")[0]
	if ticker == "" {
		ticker = "eGLD"
	} else {
		ticker = "$" + ticker
	}
	text += fmt.Sprintf("`Ticket price:` %s %s\n", utils.NicePrice(contractInfo.TicketPrice, -1), ticker)
	if contractInfo.Status != utils.StatusIdle {
		text += fmt.Sprintf("`Tickets bought:` %v\n", contractInfo.RoundTickets)
	}
	text += fmt.Sprintf("`Numbers to extract:` %v\n", contractInfo.NumbersToExtract)
	text += fmt.Sprintf("`Prize multipliers:`\n`   Bingo:` x%v\n`   Two lines:` x%v\n`   One line:` x%v\n",
		contractInfo.PrizesMultipliers.Bingo, contractInfo.PrizesMultipliers.TwoLines, contractInfo.PrizesMultipliers.OneLine)
	text += fmt.Sprintf("`Round duration:` %v seconds\n", contractInfo.RoundDuration)
	text += fmt.Sprintf("`Status:` %v\n", utils.GameStatus[contractInfo.Status])
	if contractInfo.Status == utils.StatusRunning {
		text += fmt.Sprintf("`Deadline:` %v\n", time.Unix(contractInfo.Deadline, 0))
	}
	if user != nil {
		if contractInfo.Status != utils.StatusIdle {
			tickets, err := b.networkManager.GetPlayerTickets(user.Wallet)
			if err == nil && len(tickets) > 0 {
				if len(tickets) > 1 {
					text += fmt.Sprintf("\nYou have `%v` tickets", len(tickets))
				} else {
					text += "\nYou have `1` ticket"
				}
			}
		}
	}

	return text
}

func (b *Bot) sendGameInfo(user *data.User) (tgbotapi.Message, error) {
	text := b.gameInfo(user)
	if user == nil {
		return b.sendToGroup(text)
	}

	return b.sendMessage(user.ID, text)
}

func (b *Bot) buyTicket(user *data.User, count int) {
	if contractInfo == nil {
		return
	}

	if contractInfo.Status == utils.StatusExtracting {
		b.sendMessage(user.ID, "⌛️ Please wait for the current round to finish")
		return
	}

	if contractInfo.Status == utils.StatusPaused {
		b.sendMessage(user.ID, "⏸ Contract is paused")
		return
	}

	balance, err := b.networkManager.GetBalance(user.Wallet)
	if err != nil {
		b.sendMessage(user.ID, "❗️ Network error. Please contact an administrator ("+err.Error()+")")
		return
	}

	var tokenProps *data.ESDT
	if contractInfo.TickerIdentifier == "" {
		if balance < (contractInfo.TicketPrice+utils.BuyTicketFee)*float64(count) {
			b.sendMessage(user.ID, fmt.Sprintf("⛔️ Not enough balance. You have %s eGLD and you need %s for the ticket(s) and %s for the transaction fee(s)",
				utils.NicePrice(balance, -1), utils.NicePrice(contractInfo.TicketPrice*float64(count), -1), utils.NicePrice(utils.BuyTicketFee*float64(count), -1)))
			return
		}
	} else {
		tokenBalance, err := b.networkManager.GetTokenBalance(user.Wallet, contractInfo.TickerIdentifier)
		if err != nil {
			b.sendMessage(user.ID, "❗️ Network error. Please contact an administrator ("+err.Error()+")")
			return
		}
		if balance < utils.BuyTicketFee*float64(count) {
			b.sendMessage(user.ID, fmt.Sprintf("⛔️ Not enough balance. You have %s eGLD and you need %s for the transaction fee(s)",
				utils.NicePrice(balance, -1), utils.NicePrice(utils.BuyTicketFee*float64(count), -1)))
			return
		}
		if tokenBalance < contractInfo.TicketPrice*float64(count) {
			b.sendMessage(user.ID, fmt.Sprintf("⛔️ Not enough balance. You have %s %s and you need %s for the ticket(s)",
				utils.NicePrice(tokenBalance, -1), contractInfo.TickerIdentifier, utils.NicePrice(contractInfo.TicketPrice*float64(count), -1)))
			return
		}
		tokenProps, err = b.networkManager.GetTokenProperties(contractInfo.TickerIdentifier)
		if err != nil {
			b.reportError("buyTicket - can not get token properties")
			return
		}
	}

	round := contractInfo.GameRound
	if contractInfo.Status == utils.StatusIdle {
		round++
	}

	pk := utils.GetPrivateKeyFromSeed(user.ID)

	address := utils.GetAddressFromPrivateKey(pk)
	nonce, err := b.networkManager.GetAddressNonce(address)
	if err != nil {
		nonce = utils.AutoNonce
	}

	for i := 0; i < count; i++ {
		value := contractInfo.TicketPrice
		data := "buy_ticket"
		if contractInfo.TickerIdentifier != "" {
			fValue := big.NewFloat(value)
			for j := 0; j < int(tokenProps.Decimals); j++ {
				fValue.Mul(fValue, big.NewFloat(10))
			}
			iValue, _ := fValue.Int(nil)
			sValue := hex.EncodeToString(iValue.Bytes())
			value = 0
			data = fmt.Sprintf("ESDTTransfer@%s@%s@%s", hex.EncodeToString([]byte(contractInfo.TickerIdentifier)), sValue, hex.EncodeToString([]byte(data)))
		}
		hash, err := b.networkManager.SendTransaction(pk, value, utils.BuyTicketGasLimit, data, nonce)
		nonce++
		if err != nil {
			b.sendMessage(user.ID, fmt.Sprintf("⛔️ Error sending transaction: %s", err))
			return
		}

		format := fmt.Sprintf("`Buy ticket in round #%v` - Status: ", round)
		msg := tgbotapi.NewMessage(user.ID, fmt.Sprintf("%s[pending ⌛️](%s%s)", format, b.cfg.Network.ExplorerTransaction, hash))
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.DisableWebPagePreview = true
		res, err := b.tgBot.Send(msg)
		if err != nil {
			log.Warn("can not send buy ticket tx status message", "message", msg.Text, "error", err)
			return
		}

		go b.watchBuyTicketTx(hash, format, user.ID, res.MessageID)
	}
}

func (b *Bot) watchBuyTicketTx(hash string, format string, chatID int64, messageID int) {
	for {
		time.Sleep(time.Second * 5)
		info, err := b.networkManager.GetTransactionInfo(hash)
		if err != nil {
			continue
		}
		status := info.Source.Status
		switch status {
		case "pending":
			status += " ⌛️"
		case "success":
			status += " ✅"
		default:
			status += " ❌"
		}

		if info.Source.Status != "pending" {
			msg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s[%s](%s%s)",
				format, status, b.cfg.Network.ExplorerTransaction, hash))
			msg.ParseMode = tgbotapi.ModeMarkdown
			msg.DisableWebPagePreview = true
			b.tgBot.Send(msg)
			if info.Source.Status == "success" {
				scrs, err := b.networkManager.GetTransactionScrs(hash)
				if err != nil {
					time.Sleep(time.Second * 6)
					scrs, err = b.networkManager.GetTransactionScrs(hash)
				}
				if err == nil {
					rawTicket := string(scrs[0].Source.Data)
					params := strings.Split(rawTicket, "@")
					rawTicket = params[len(params)-1]
					ticket, err := hex.DecodeString(rawTicket)
					if err == nil {
						ticket3x9 := utils.BytesToTicket(ticket)
						res, err := b.sendMessage(chatID, fmt.Sprintf("%v", utils.FormatTicket(ticket3x9)))
						user, ok := b.users[chatID]
						if err == nil && user != nil && ok {
							user.Tickets = append(user.Tickets, &data.TelegramTicket{
								Ticket:    ticket3x9,
								MessageID: res.MessageID,
							})
						}
					}
				} else {
					log.Warn("watchBuyTicketTx - GetTransactionScrs", "error", err, "hash", hash)
				}
			}
			return
		}
	}
}

func (b *Bot) watchExtractNumbersTx(hash string, format string, chatID int64, messageID int) {
	fDenom := big.NewFloat(1)
	if contractInfo.TickerIdentifier != "" {
		prop, err := b.networkManager.GetTokenProperties(contractInfo.TickerIdentifier)
		if err != nil {
			log.Error("getTokenBalance - getTokenProperties", "error", err)
			return
		}

		for i := 0; i < int(prop.Decimals); i++ {
			fDenom.Mul(fDenom, big.NewFloat(10))
		}
	}

	for {
		time.Sleep(time.Second * 5)
		info, err := b.networkManager.GetTransactionInfo(hash)
		if err != nil {
			continue
		}
		status := info.Source.Status
		switch status {
		case "pending":
			status += " ⌛️"
		case "success":
			status += " ✅"
		default:
			status += " ❌"
		}

		if info.Source.Status != "pending" {
			msg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("%s[%s](%s%s)",
				format, status, b.cfg.Network.ExplorerTransaction, hash))
			msg.ParseMode = tgbotapi.ModeMarkdown
			msg.DisableWebPagePreview = true
			b.tgBot.Send(msg)
			if info.Source.Status == "success" {
				scrs, err := b.networkManager.GetTransactionScrs(hash)
				if err != nil {
					time.Sleep(time.Second * 6)
					scrs, err = b.networkManager.GetTransactionScrs(hash)
				}
				if err == nil {
					winners := make(map[string][]*data.Telegram)
					for _, scr := range scrs {
						if string(scr.Source.Data) == "@6f6b" {
							continue
						}
						user := b.getUserByAddress(scr.Source.Receiver)
						if user != nil {
							tgUser := b.tgUsers[user.ID]
							paramsList := strings.Split(string(scr.Source.Data), "@")
							fValue, _ := big.NewFloat(0).SetString(scr.Source.Value)
							fValue.Quo(fValue, b.networkManager.GetEgldDenomination())
							value, _ := fValue.Float64()
							if len(paramsList) > 0 && contractInfo.TickerIdentifier != "" {
								won := paramsList[len(paramsList)-1]
								bWon, err := hex.DecodeString(won)
								if err == nil {
									iValue := big.NewInt(0).SetBytes(bWon)
									fValue := big.NewFloat(0).SetInt(iValue)
									fValue.Quo(fValue, fDenom)
									value, _ = fValue.Float64()
								}
							}
							prize := "💰"
							if value == contractInfo.TicketPrice*float64(contractInfo.PrizesMultipliers.OneLine) {
								prize = "Line!"
							}
							if value == contractInfo.TicketPrice*float64(contractInfo.PrizesMultipliers.TwoLines) {
								prize = "2 Lines! 🥳"
							}
							if value == contractInfo.TicketPrice*float64(contractInfo.PrizesMultipliers.Bingo) {
								prize = "Bingo! 💥💥💥"
							}
							if winners[prize] == nil {
								winners[prize] = make([]*data.Telegram, 0)
							}
							winners[prize] = append(winners[prize], tgUser)
						}
					}
					text := ""
					for prize, users := range winners {
						unique := make(map[*data.Telegram]int)
						line := ""
						for _, user := range users {
							unique[user]++
						}
						i := 0
						for user, count := range unique {
							if count == 0 {
								continue
							}
							text := "🤑 You won " + prize
							if i > 0 {
								line += ", "
							}
							name := utils.FormatDbTgUser(user)
							line += name
							if count > 1 {
								line += fmt.Sprintf(" (x%v)", count)
								text += fmt.Sprintf(" (x%v)", count)
							}
							b.sendMessage(user.ID, text)
							i++
						}
						if len(unique) == 1 {
							line += " has"
						} else {
							line += " have"
						}
						line += " won " + prize + "\n"
						text += line
					}
					if text == "" {
						text = "😔 Nobody won"
					}
					text = strings.ReplaceAll(text, "_", "\\_")
					b.sendToGroup(text)
				} else {
					log.Warn("watchExtractNumbersTx - GetTransactionScrs", "error", err, "hash", hash)
				}
			}
			b.strikethroughTickets()
			return
		}
	}
}

func (b *Bot) strikethroughTickets() {
	lastExtracted, err := b.networkManager.GetLastExtractedNumbers()
	if err != nil {
		log.Warn("strikethrough - error getting last extracted numbers", "error", err)
		return
	}

	for _, user := range b.users {
		if len(user.Tickets) == 0 {
			continue
		}

		for _, ticket := range user.Tickets {
			newText := fmt.Sprintf("%v", utils.FormatTicketStrikethrough(ticket.Ticket, lastExtracted))
			msg := tgbotapi.NewEditMessageText(user.ID, ticket.MessageID, newText)
			msg.ParseMode = "MarkdownV2"
			b.tgBot.Send(msg)
		}
		user.Tickets = make([]*data.TelegramTicket, 0)
	}
}

func (b *Bot) sendMyTickets(user *data.User) {
	if contractInfo.Status != utils.StatusRunning {
		b.sendMessage(user.ID, "❕ Game must be running")
		return
	}

	tickets, err := b.networkManager.GetPlayerTickets(user.Wallet)
	if err != nil {
		log.Warn("can not get player tickets", "error", err)
		return
	}

	if len(tickets) == 0 {
		b.sendMessage(user.ID, "🚫 You have no tickets in this round")
		return
	}

	for _, ticket := range user.Tickets {
		msg := tgbotapi.NewDeleteMessage(user.ID, ticket.MessageID)
		b.tgBot.Send(msg)
	}

	user.Tickets = make([]*data.TelegramTicket, 0)
	for _, ticket := range tickets {
		res, err := b.sendMessage(user.ID, fmt.Sprintf("%v", utils.FormatTicket(ticket)))
		if err == nil {
			user.Tickets = append(user.Tickets, &data.TelegramTicket{
				Ticket:    ticket,
				MessageID: res.MessageID,
			})
		}
	}
}

func (b *Bot) sendStatistics(user *data.User) {
	tickets, bingo, two, one, err := b.networkManager.GetStatistics()
	if err != nil {
		return
	}

	ftickets := float64(tickets)
	fbingo := float64(bingo)
	ftwo := float64(two) + fbingo
	fone := float64(one) + ftwo
	text := fmt.Sprintf("`Statistics`\n\n`Tickets:` %v\n`Bingo:` %v (%.2f%%)\n`2 Lines:` %v (%.2f%%)\n`1 Line:` %v (%.2f%%)",
		tickets, bingo, fbingo*100/ftickets, two, ftwo*100/ftickets, one, fone*100/ftickets)
	b.sendMessage(user.ID, text)
}

func (b *Bot) getOrCreateUser(tgUser *tgbotapi.User) *data.User {
	id := int64(tgUser.ID)
	user, ok := b.users[id]
	if !ok {
		user = &data.User{
			ID:      id,
			Wallet:  utils.GetAddressFromPrivateKey(utils.GetPrivateKeyFromSeed(id)),
			Tickets: make([]*data.TelegramTicket, 0),
		}
		b.users[id] = user
	}

	tg, ok := b.tgUsers[id]
	if !ok || tg.UserName != tgUser.UserName || tg.FirstName != tgUser.FirstName || tg.LastName != tgUser.LastName {
		b.tgUsers[id] = &data.Telegram{
			ID:        id,
			UserName:  tgUser.UserName,
			FirstName: tgUser.FirstName,
			LastName:  tgUser.LastName,
		}
	}

	return user
}

func (b *Bot) getUserByAddress(address string) *data.User {
	for _, user := range b.users {
		if user.Wallet == address {
			return user
		}
	}

	return nil
}
