package data

type ContractInfo struct {
	GameRound            uint64
	RoundTickets         uint64
	NumbersToExtract     uint64
	LastExtractedNumbers []byte
	TicketPrice          float64
	PrizesMultipliers    struct {
		Bingo    uint64
		TwoLines uint64
		OneLine  uint64
	}
	Deadline      int64
	RoundDuration int64
	Status        int
	Statistics    struct {
		TotalTickets  uint64
		TotalBingo    uint64
		TotalTwoLines uint64
		TotalOneLine  uint64
	}
	TickerIdentifier string
}
