package config

import (
	"encoding/json"
	"io/ioutil"

	"github.com/DrDelphi/EsdtBingoBot/data"
)

var (
	cfgPath string
)

// NewConfig - reads the application configuration from the provided path
// and returns an AppConfig struct or an error if something goes wrong
func NewConfig(configPath string) (*data.AppConfig, error) {
	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := &data.AppConfig{}
	err = json.Unmarshal(bytes, cfg)
	if err != nil {
		return nil, err
	}

	cfgPath = configPath

	return cfg, nil
}

func Save(cfg *data.AppConfig) error {
	bytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(cfgPath, bytes, 0644)
}
