package data

type User struct {
	ID      int64
	Wallet  string
	Tickets []*TelegramTicket
}

type Telegram struct {
	ID        int64
	UserName  string
	FirstName string
	LastName  string
}

type TelegramTicket struct {
	Ticket    *Ticket
	MessageID int
}
