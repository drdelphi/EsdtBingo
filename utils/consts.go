package utils

const (
	DefaultConfigPath = "config.json"

	AutoNonce = 4000000000

	StatusRunning    = 0
	StatusExtracting = 1
	StatusIdle       = 2
	StatusPaused     = 3

	BuyTicketFee           = 0.0003
	BuyTicketGasLimit      = 10000000
	ExtractNumbersGasLimit = 40000000
)

var (
	Seedphrase string
	GameStatus = []string{"Running", "Extracting", "Idle", "Paused"}
)
