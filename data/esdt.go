package data

type ESDT struct {
	Name        string
	Ticker      string
	ShortTicker string
	Decimals    uint64
	Price       float64
}

type TokensList struct {
	Data struct {
		Tokens []string `json:"tokens"`
	} `json:"data"`
}

type EsdtBalanceResponse struct {
	Data struct {
		TokenData EsdtBalance `json:"tokenData"`
	} `json:"data"`
}

type EsdtBalance struct {
	Balance string `json:"balance"`
}
