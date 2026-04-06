package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"network_monitor_tool/utils"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type PingResult struct {
	Host   string
	Pass   bool
	TimeMs int64
	Raw    string
}

func PingHosts(hosts []string) {
	if len(hosts) == 0 {
		return
	}

	for _, host := range hosts {
		result := PingHost(host)
		timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")

		utils.PrintColor("blue", "Ping result: host="+result.Host+" pass="+strconv.Itoa(boolToInt(result.Pass))+" timeMs="+strconv.FormatInt(result.TimeMs, 10)+" timestamp="+timestamp+"\n")

		dbManager := utils.GetDatabase()
		conn, err := dbManager.GetConnection()
		if err != nil {
			utils.PrintColor("red", "DB connection error: "+err.Error())
			continue
		}

		insertSQL := `INSERT INTO ping_results (host, pass, time_ms, timestamp) VALUES (?, ?, ?, ?)`
		_, err = conn.Exec(insertSQL, result.Host, boolToInt(result.Pass), result.TimeMs, timestamp)
		if err != nil {
			utils.PrintColor("red", "DB insert error:"+err.Error())
		}

		conn.Close()
	}
}

func PingHost(host string) PingResult {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", host)
	} else {
		cmd = exec.Command("ping", "-c", "1", host)
	}

	output, err := cmd.Output()
	success := false
	timeMs := int64(-1)
	raw := string(output)

	if err == nil {
		lines := strings.Split(strings.ToLower(raw), "\n")
		for _, line := range lines {
			if strings.Contains(line, "time=") {
				success = true
				timeMs = int64(math.Round(extractTimeMs(line)))
				break
			}
		}
	}

	return PingResult{
		Host:   host,
		Pass:   success,
		TimeMs: timeMs,
		Raw:    raw,
	}
}

func extractTimeMs(line string) float64 {
	idx := strings.Index(line, "time=")
	if idx == -1 {
		return -1
	}
	line = line[idx+5:]
	endIdx := strings.Index(line, "ms")
	if endIdx == -1 {
		endIdx = strings.Index(line, " ")
	}
	if endIdx == -1 {
		endIdx = len(line)
	}
	timeStr := strings.TrimSpace(line[:endIdx])
	timeStr = strings.ReplaceAll(timeStr, "=", "")

	val, err := strconv.ParseFloat(timeStr, 64)
	if err != nil {
		return -1
	}
	return val
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type PingDBResult struct {
	Host      string `json:"host"`
	Success   string `json:"success"`
	TimeMs    string `json:"timems"`
	Timestamp string `json:"timestamp"`
}

func GetAllPingResults(w http.ResponseWriter, r *http.Request) {
	dbManager := utils.GetDatabase()
	conn, err := dbManager.GetConnection()
	if err != nil {
		http.Error(w, "DB connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	rows, err := conn.Query("SELECT host, pass, time_ms, timestamp FROM ping_results")
	if err != nil {
		http.Error(w, "DB query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []PingDBResult{}
	for rows.Next() {
		var host string
		var pass bool
		var timeMs int64
		var timestamp string

		err := rows.Scan(&host, &pass, &timeMs, &timestamp)
		if err != nil {
			utils.PrintColor("green", "Row scan error:"+err.Error())
			continue
		}

		results = append(results, PingDBResult{
			Host:      host,
			Success:   fmt.Sprintf("%v", pass),
			TimeMs:    fmt.Sprintf("%d", timeMs),
			Timestamp: timestamp,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
