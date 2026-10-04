package tools

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"network_monitor_tool/utils"
)

// SpeedtestResult is normalized across engines. Download/Upload are bytes per second,
// which is what the Ookla CLI reports and what the database has always stored.
type SpeedtestResult struct {
	Download       float64
	Upload         float64
	Ping           float64
	Jitter         *float64
	PacketLoss     *float64
	ServerID       string
	ServerHost     string
	ServerLocation string
	ISP            string
	ResultURL      string
}

// SpeedtestEngine describes which speed test CLI we found.
type SpeedtestEngine struct {
	Name string `json:"name"` // "ookla", "python" or "" when nothing usable is installed
	Path string `json:"path"`
}

var (
	engineMu    sync.Mutex
	engineKey   string
	engineCache SpeedtestEngine
	engineAt    time.Time
)

// DetectSpeedtest finds the speed test CLI. "auto" prefers the official Ookla CLI and
// falls back to the Python speedtest-cli (which is what's packaged for pfSense).
func DetectSpeedtest(c utils.Config) SpeedtestEngine {
	key := c.SpeedtestEngine + "|" + c.SpeedtestPath
	engineMu.Lock()
	defer engineMu.Unlock()
	// Re-detect now and then so installing speedtest while the app runs gets noticed.
	if key == engineKey && time.Since(engineAt) < time.Minute {
		return engineCache
	}

	var candidates []string
	if c.SpeedtestPath != "" {
		candidates = []string{c.SpeedtestPath}
	} else {
		for _, name := range []string{"speedtest", "speedtest-cli"} {
			if p, ok := utils.LookCommand(name); ok {
				candidates = append(candidates, p)
			}
		}
	}

	found := SpeedtestEngine{}
	for _, path := range candidates {
		kind := identifySpeedtest(path)
		if kind == "" {
			continue
		}
		if c.SpeedtestEngine == "auto" || c.SpeedtestEngine == kind {
			found = SpeedtestEngine{Name: kind, Path: path}
			if kind == "ookla" || c.SpeedtestEngine != "auto" {
				break
			}
		}
	}

	engineKey, engineCache, engineAt = key, found, time.Now()
	return found
}

func identifySpeedtest(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	text := strings.ToLower(string(out))
	switch {
	case strings.Contains(text, "ookla"):
		return "ookla"
	case strings.Contains(text, "speedtest-cli") || strings.Contains(text, "python"):
		return "python"
	}
	return ""
}

func RunSpeedtest(c utils.Config) error {
	engine := DetectSpeedtest(c)
	if engine.Name == "" {
		return errors.New("no speedtest CLI found - install it from https://www.speedtest.net/apps/cli")
	}
	utils.PrintColor("yellow", "Running speed test ("+engine.Name+")...")

	var args []string
	if engine.Name == "ookla" {
		// Accepting the license up front is what lets this run unattended as a service.
		args = []string{"--accept-license", "--accept-gdpr", "--format=json", "--progress=no"}
		if c.SpeedtestServerID != "" {
			args = append(args, "--server-id="+c.SpeedtestServerID)
		}
	} else {
		args = []string{"--json", "--secure"}
		if c.SpeedtestServerID != "" {
			args = append(args, "--server", c.SpeedtestServerID)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, engine.Path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(string(output))
		}
		utils.PrintColor("red", "Error running speedtest: "+err.Error()+" "+msg)
		return fmt.Errorf("speedtest failed: %v %s", err, lastLine(msg))
	}

	var result SpeedtestResult
	if engine.Name == "ookla" {
		result, err = ParseOoklaOutput(output)
	} else {
		result, err = ParsePythonSpeedtestOutput(output)
	}
	if err != nil {
		utils.PrintColor("red", "Speedtest parse error: "+err.Error())
		return err
	}

	utils.PrintColor("green", fmt.Sprintf("Speedtest: download %.1f Mbps, upload %.1f Mbps, ping %.1f ms, server %s (%s)",
		result.Download*8/1e6, result.Upload*8/1e6, result.Ping, result.ServerLocation, result.ServerID))
	if err := saveSpeedtestToDB(result); err != nil {
		utils.PrintColor("red", "DB insert error: "+err.Error())
		return err
	}
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

type ooklaJSON struct {
	Type string `json:"type"`
	Ping struct {
		Jitter  *float64 `json:"jitter"`
		Latency float64  `json:"latency"`
	} `json:"ping"`
	Download *struct {
		Bandwidth float64 `json:"bandwidth"`
	} `json:"download"`
	Upload *struct {
		Bandwidth float64 `json:"bandwidth"`
	} `json:"upload"`
	PacketLoss *float64 `json:"packetLoss"`
	ISP        string   `json:"isp"`
	Server     struct {
		ID       json.Number `json:"id"`
		Host     string      `json:"host"`
		Name     string      `json:"name"`
		Location string      `json:"location"`
		Country  string      `json:"country"`
	} `json:"server"`
	Result struct {
		URL string `json:"url"`
	} `json:"result"`
}

// ParseOoklaOutput reads `speedtest --format=json`. The CLI can print log lines as
// JSON objects too, so pick the line that is the actual result.
func ParseOoklaOutput(output []byte) (SpeedtestResult, error) {
	for _, line := range bytes.Split(output, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var o ooklaJSON
		if err := json.Unmarshal(line, &o); err != nil {
			continue
		}
		if o.Download == nil || o.Upload == nil || (o.Type != "" && o.Type != "result") {
			continue
		}
		location := o.Server.Location
		if o.Server.Country != "" && location != "" {
			location += ", " + o.Server.Country
		}
		return SpeedtestResult{
			Download:       o.Download.Bandwidth,
			Upload:         o.Upload.Bandwidth,
			Ping:           o.Ping.Latency,
			Jitter:         o.Ping.Jitter,
			PacketLoss:     o.PacketLoss,
			ServerID:       o.Server.ID.String(),
			ServerHost:     o.Server.Host,
			ServerLocation: location,
			ISP:            o.ISP,
			ResultURL:      o.Result.URL,
		}, nil
	}
	return SpeedtestResult{}, errors.New("no result in speedtest output")
}

type pythonJSON struct {
	Download float64 `json:"download"` // bits per second
	Upload   float64 `json:"upload"`
	Ping     float64 `json:"ping"`
	Share    string  `json:"share"`
	Server   struct {
		ID      json.Number `json:"id"`
		Host    string      `json:"host"`
		Name    string      `json:"name"`
		Country string      `json:"country"`
		Sponsor string      `json:"sponsor"`
	} `json:"server"`
	Client struct {
		ISP string `json:"isp"`
	} `json:"client"`
}

// ParsePythonSpeedtestOutput reads `speedtest-cli --json`.
func ParsePythonSpeedtestOutput(output []byte) (SpeedtestResult, error) {
	start := bytes.IndexByte(output, '{')
	if start < 0 {
		return SpeedtestResult{}, errors.New("no JSON in speedtest-cli output")
	}
	var p pythonJSON
	if err := json.Unmarshal(output[start:], &p); err != nil {
		return SpeedtestResult{}, err
	}
	location := p.Server.Name
	if p.Server.Country != "" {
		location += ", " + p.Server.Country
	}
	return SpeedtestResult{
		Download:       p.Download / 8,
		Upload:         p.Upload / 8,
		Ping:           p.Ping,
		ServerID:       p.Server.ID.String(),
		ServerHost:     p.Server.Host,
		ServerLocation: location,
		ISP:            p.Client.ISP,
		ResultURL:      p.Share,
	}, nil
}

func saveSpeedtestToDB(r SpeedtestResult) error {
	conn, err := db()
	if err != nil {
		return err
	}
	var serverID any
	if id, err := strconv.ParseInt(r.ServerID, 10, 64); err == nil {
		serverID = id
	}
	_, err = conn.Exec(`INSERT INTO speedtest_results
	(download, upload, ping, jitter, packet_loss, server_id, server_host, server_location, isp, result_url, timestamp)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Download, r.Upload, r.Ping, r.Jitter, r.PacketLoss, serverID, r.ServerHost, r.ServerLocation, r.ISP, r.ResultURL,
		utils.DBTime(time.Now()))
	return err
}

type SpeedtestRow struct {
	ID             int64    `json:"id"`
	DownloadMbps   float64  `json:"download_mbps"`
	UploadMbps     float64  `json:"upload_mbps"`
	PingMs         float64  `json:"ping_ms"`
	JitterMs       *float64 `json:"jitter_ms"`
	PacketLoss     *float64 `json:"packet_loss"`
	ServerID       string   `json:"server_id"`
	ServerHost     string   `json:"server_host"`
	ServerLocation string   `json:"server_location"`
	ISP            string   `json:"isp"`
	ResultURL      string   `json:"result_url"`
	Timestamp      string   `json:"timestamp"`
}

func toMbps(bytesPerSecond float64) float64 { return bytesPerSecond * 8 / 1e6 }

// QuerySpeedtests returns speed tests in the range, oldest first. limit > 0 returns
// only the newest `limit` rows.
func QuerySpeedtests(r TimeRange, limit int) ([]SpeedtestRow, error) {
	conn, err := db()
	if err != nil {
		return nil, err
	}
	where, args := r.where("timestamp")
	query := `SELECT id, download, upload, ping, jitter, packet_loss, server_id, server_host, server_location, isp, result_url, timestamp
		FROM speedtest_results` + where
	if limit > 0 {
		query += " ORDER BY timestamp DESC, id DESC LIMIT " + strconv.Itoa(limit)
	} else {
		query += " ORDER BY timestamp, id"
	}
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []SpeedtestRow{}
	for rows.Next() {
		var s SpeedtestRow
		var download, upload, ping float64
		var jitter, loss sql.NullFloat64
		var serverID, serverHost, serverLoc, isp, url sql.NullString
		var ts any
		if err := rows.Scan(&s.ID, &download, &upload, &ping, &jitter, &loss, &serverID, &serverHost, &serverLoc, &isp, &url, &ts); err != nil {
			utils.PrintColor("red", "Row scan error: "+err.Error())
			continue
		}
		s.DownloadMbps = toMbps(download)
		s.UploadMbps = toMbps(upload)
		s.PingMs = ping
		if jitter.Valid {
			s.JitterMs = &jitter.Float64
		}
		if loss.Valid {
			s.PacketLoss = &loss.Float64
		}
		s.ServerID, s.ServerHost, s.ServerLocation = serverID.String, serverHost.String, serverLoc.String
		s.ISP, s.ResultURL = isp.String, url.String
		s.Timestamp = utils.FormatDBTime(ts)
		results = append(results, s)
	}
	if limit > 0 {
		for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
			results[i], results[j] = results[j], results[i]
		}
	}
	return results, rows.Err()
}
