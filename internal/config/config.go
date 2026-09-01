package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DBURL    string `json:"db_url"`
	Username string `json:"current_user_name"`
}

const configFileName = ".gatorconfig.json"

func Read() (*Config, error) {
	return read(configFileName)
}

func read(filename string) (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(homeDir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func write(cfg *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	filePath := filepath.Join(homeDir, configFileName)
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0o644)
}

func SetUser(username string) error {
	config, err := Read()
	if err != nil {
		return err
	}

	config.Username = username

	return write(config)
}
