package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	TracerouteHost    []string `json:"traceroute_host"`
	PingHost          []string `json:"ping_host"`
	SpeedtestServerID string   `json:"speedtest_server_id"`
	RunSQL            bool     `json:"run_sql"`
	SQLUser           string   `json:"sql_user"`
	SQLPassword       string   `json:"sql_password"`
	SQLHost           string   `json:"sql_host"`
	SQLPort           string   `json:"sql_port"`
	SQLDatabase       string   `json:"sql_database"`
	RunEvery          string   `json:"run_every"`
}

const configFile = "config.json"

var Cfg *Config

func LoadConfig() error {
	var cfg Config

	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		cfg = Config{
			TracerouteHost:    []string{"google.com", "github.com"},
			PingHost:          []string{"google.com", "github.com", "1.1.1.1", "8.8.8.8"},
			SpeedtestServerID: "",
			RunSQL:            false,
			SQLUser:           "user",
			SQLPassword:       "password",
			SQLHost:           "localhost",
			SQLPort:           "3306",
			SQLDatabase:       "my database name",
			RunEvery:          "1h",
		}

		file, err := os.Create(configFile)
		if err != nil {
			return fmt.Errorf("failed to create config: %v", err)
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(cfg); err != nil {
			return fmt.Errorf("failed to write default config: %v", err)
		}

		PrintColor("cyan", "Created default config.json")
	} else {
		file, err := os.Open(configFile)
		if err != nil {
			return fmt.Errorf("failed to open config: %v", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&cfg); err != nil {
			return fmt.Errorf("failed to parse config: %v", err)
		}
	}

	Cfg = &cfg
	return nil
}
