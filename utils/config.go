package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Config struct {
	RunPing       bool `json:"run_ping"`
	Runtraceroute bool `json:"run_traceroute"`
	Runspeedtest  bool `json:"run_speedtest"`

	PingHost       []string `json:"ping_host"`
	PingCount      int      `json:"ping_count"`
	TracerouteHost []string `json:"traceroute_host"`

	SpeedtestServerID string `json:"speedtest_server_id"`
	SpeedtestEngine   string `json:"speedtest_engine"` // auto | ookla | python
	SpeedtestPath     string `json:"speedtest_path"`   // optional explicit path to the speedtest binary

	// What you pay your ISP for, used to show "% of plan" on the dashboard (0 = unset).
	PlanDownloadMbps float64 `json:"plan_download_mbps"`
	PlanUploadMbps   float64 `json:"plan_upload_mbps"`

	RunEvery      string `json:"run_every"`
	AlignToClock  bool   `json:"align_to_clock"` // "1h" runs at :00 instead of an hour after startup
	RetentionDays int    `json:"retention_days"` // 0 = keep forever

	Listen     string `json:"listen"`
	UIPassword string `json:"ui_password"` // optional HTTP basic auth for the web UI

	RunSQL      bool   `json:"run_sql"`
	SQLUser     string `json:"sql_user"`
	SQLPassword string `json:"sql_password"`
	SQLHost     string `json:"sql_host"`
	SQLPort     string `json:"sql_port"`
	SQLDatabase string `json:"sql_database"`
}

func DefaultConfig() Config {
	return Config{
		RunPing:         true,
		Runtraceroute:   true,
		Runspeedtest:    true,
		PingHost:        []string{"google.com", "github.com", "1.1.1.1", "8.8.8.8"},
		PingCount:       4,
		TracerouteHost:  []string{"google.com", "github.com"},
		SpeedtestEngine: "auto",
		RunEvery:        "1h",
		AlignToClock:    true,
		RetentionDays:   0,
		Listen:          ":8080",
		SQLUser:         "user",
		SQLPassword:     "password",
		SQLHost:         "localhost",
		SQLPort:         "3306",
		SQLDatabase:     "my_database_name",
	}
}

var (
	cfgMu      sync.RWMutex
	cfg        Config
	configPath string
)

// GetConfig returns a copy of the current config.
func GetConfig() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	c := cfg
	c.PingHost = append([]string(nil), cfg.PingHost...)
	c.TracerouteHost = append([]string(nil), cfg.TracerouteHost...)
	return c
}

func ConfigPath() string {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return configPath
}

// LoadConfig reads config.json, filling in defaults for any missing options, and
// writes it back so new options show up in the file. created reports a first run.
func LoadConfig() (created bool, err error) {
	dir, err := DataDir()
	if err != nil {
		return false, fmt.Errorf("cannot get data dir: %v", err)
	}
	path := filepath.Join(dir, "config.json")

	c := DefaultConfig()
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		created = true
	case err != nil:
		return false, fmt.Errorf("failed to read config: %v", err)
	default:
		if err := json.Unmarshal(data, &c); err != nil {
			return false, fmt.Errorf("failed to parse %s: %v", path, err)
		}
	}

	c.Normalize()
	if err := c.Validate(); err != nil {
		return false, fmt.Errorf("invalid config %s: %v", path, err)
	}

	cfgMu.Lock()
	configPath = path
	cfg = c
	cfgMu.Unlock()

	if newData, _ := marshalConfig(c); !bytes.Equal(newData, data) {
		if err := writeConfig(path, c); err != nil {
			return created, err
		}
		if created {
			PrintColor("cyan", "Created default config.json at "+path)
		}
	}
	return created, nil
}

// SaveConfig validates, writes and activates a new config.
func SaveConfig(c Config) error {
	c.Normalize()
	if err := c.Validate(); err != nil {
		return err
	}
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if err := writeConfig(configPath, c); err != nil {
		return err
	}
	cfg = c
	return nil
}

func marshalConfig(c Config) ([]byte, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// writeConfig writes atomically so a crash mid-save can't leave a broken config.
func writeConfig(path string, c Config) error {
	data, err := marshalConfig(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}
	return nil
}

func cleanList(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// Normalize trims and de-duplicates values and fills in defaults for empty ones.
func (c *Config) Normalize() {
	c.PingHost = cleanList(c.PingHost)
	c.TracerouteHost = cleanList(c.TracerouteHost)
	c.SpeedtestServerID = strings.TrimSpace(c.SpeedtestServerID)
	c.SpeedtestEngine = strings.ToLower(strings.TrimSpace(c.SpeedtestEngine))
	if c.SpeedtestEngine == "" {
		c.SpeedtestEngine = "auto"
	}
	c.SpeedtestPath = strings.TrimSpace(c.SpeedtestPath)
	c.RunEvery = strings.ToLower(strings.TrimSpace(c.RunEvery))
	c.Listen = strings.TrimSpace(c.Listen)
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.PingCount == 0 {
		c.PingCount = 4
	}
}

// Hosts end up as command arguments, so only allow hostname/IP characters and never
// a leading "-" (which ping/traceroute would read as a flag).
var hostRe = regexp.MustCompile(`^[A-Za-z0-9_\[][A-Za-z0-9._:\-\[\]%]*$`)
var digitsRe = regexp.MustCompile(`^\d+$`)

func (c Config) Validate() error {
	interval, err := ParseInterval(c.RunEvery)
	if err != nil {
		return fmt.Errorf("run_every: %v", err)
	}
	if interval < 30*time.Second {
		return fmt.Errorf("run_every must be at least 30s")
	}
	if c.PingCount < 1 || c.PingCount > 20 {
		return fmt.Errorf("ping_count must be between 1 and 20")
	}
	for _, h := range append(append([]string{}, c.PingHost...), c.TracerouteHost...) {
		if len(h) > 253 || !hostRe.MatchString(h) {
			return fmt.Errorf("invalid host %q", h)
		}
	}
	if c.SpeedtestServerID != "" && !digitsRe.MatchString(c.SpeedtestServerID) {
		return fmt.Errorf("speedtest_server_id must be a number")
	}
	switch c.SpeedtestEngine {
	case "auto", "ookla", "python":
	default:
		return fmt.Errorf("speedtest_engine must be auto, ookla or python")
	}
	if c.PlanDownloadMbps < 0 || c.PlanUploadMbps < 0 {
		return fmt.Errorf("plan speeds can't be negative")
	}
	if c.RetentionDays < 0 {
		return fmt.Errorf("retention_days can't be negative")
	}
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return fmt.Errorf("listen must look like :8080 or 127.0.0.1:8080")
	}
	if c.RunSQL && (c.SQLHost == "" || c.SQLDatabase == "") {
		return fmt.Errorf("sql_host and sql_database are required when run_sql is on")
	}
	return nil
}

// DatabaseSettingsEqual reports whether two configs point at the same database.
func (c Config) DatabaseSettingsEqual(o Config) bool {
	if c.RunSQL != o.RunSQL {
		return false
	}
	if !c.RunSQL {
		return true
	}
	return c.SQLUser == o.SQLUser && c.SQLPassword == o.SQLPassword && c.SQLHost == o.SQLHost &&
		c.SQLPort == o.SQLPort && c.SQLDatabase == o.SQLDatabase
}
