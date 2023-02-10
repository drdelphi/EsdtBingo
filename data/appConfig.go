package data

// AppConfig holds the application configuration read from config.json
type AppConfig struct {
	Bot struct {
		Token   string `json:"token"`
		Owner   int64  `json:"owner"`
		Group   string `json:"group"`
		GroupID int64  `json:"groupID"`
	} `json:"bot"`
	Seedphrase      string `json:"seed"`
	ContractAddress string `json:"contractAddress"`
	Network         struct {
		Proxy               string `json:"proxy"`
		Indexer             string `json:"indexer"`
		ExplorerTransaction string `json:"explorerTransaction"`
		ExplorerAccount     string `json:"explorerAccount"`
	} `json:"network"`
}
