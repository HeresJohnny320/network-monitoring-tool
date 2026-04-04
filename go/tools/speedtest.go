package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"network_monitor_tool/utils"
	"os/exec"
	"strings"
	"time"
)

type SpeedtestResult struct {
	Download       string `json:"download"`
	Upload         string `json:"upload"`
	Ping           string `json:"ping"`
	ServerID       string `json:"serverId"`
	ServerHost     string `json:"serverHost"`
	ServerLocation string `json:"serverLocation"`
	Timestamp      string `json:"timestamp"`
}

func RunSpeedtest(server string) {
	utils.PrintColor("yellow", "Running speed test...")
	pythonSupport := false

	var cmd *exec.Cmd
	if !pythonSupport {
		if server != "" {
			cmd = exec.Command("speedtest", "--server-id", server, "--format=json")
		} else {
			cmd = exec.Command("speedtest", "--format=json")
		}
	} else {
		if server != "" {
			cmd = exec.Command("speedtest", "--server", server, "--json")
		} else {
			cmd = exec.Command("speedtest", "--json")
		}
	}

	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		utils.PrintColor("red", "Error running speedtest CLI:", err.Error()) // did u accept the EULA
		return
	}

	output := strings.ToLower(string(outputBytes))
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
		utils.PrintColor("red", "JSON parse error:", err.Error())
		return
	}

	var download, upload, pingStr, serverLoc string
	var serverHost, serverID string

	serverInfo := make(map[string]interface{})
	if si, ok := jsonData["server"].(map[string]interface{}); ok {
		serverInfo = si
		serverHost, _ = serverInfo["host"].(string)

		switch id := serverInfo["id"].(type) {
		case string:
			serverID = id
		case float64:
			serverID = fmt.Sprintf("%.0f", id)
		}

		serverLoc, _ = serverInfo["location"].(string)
	}

	if !pythonSupport {
		if dl, ok := jsonData["download"].(map[string]interface{}); ok {
			if bw, ok := dl["bandwidth"].(float64); ok {
				download = fmt.Sprintf("%.0f", bw)
			}
		}
		if ul, ok := jsonData["upload"].(map[string]interface{}); ok {
			if bw, ok := ul["bandwidth"].(float64); ok {
				upload = fmt.Sprintf("%.0f", bw)
			}
		}
		if p, ok := jsonData["ping"].(map[string]interface{}); ok {
			if latency, ok := p["latency"].(float64); ok {
				pingStr = fmt.Sprintf("%.2f", latency)
			}
		}
	} else {
		if val, ok := jsonData["download"].(string); ok {
			download = val
		}
		if val, ok := jsonData["upload"].(string); ok {
			upload = val
		}
		if val, ok := jsonData["ping"].(string); ok {
			pingStr = val
		}
		if loc, ok := serverInfo["name"].(string); ok {
			serverLoc = loc
		}
	}

	utils.PrintColor("green", "Speedtest ="+"download:"+download+", upload:"+upload+", ping:"+pingStr+", serverID:"+serverID+", serverHost:"+serverHost+", serverLoc:"+serverLoc)
	saveSpeedtestToDB(download, upload, pingStr, serverID, serverHost, serverLoc)
}

func saveSpeedtestToDB(download, upload, ping, serverID, serverHost, serverLoc string) {
	dbManager := utils.GetDatabase()
	conn, err := dbManager.GetConnection()
	if err != nil {
		utils.PrintColor("red", "DB connection error:"+err.Error())
		return
	}
	defer conn.Close()

	sqlStmt := `INSERT INTO speedtest_results
	(download, upload, ping, server_id, server_host, server_location, timestamp)
	VALUES (?, ?, ?, ?, ?, ?, ?)`

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = conn.Exec(sqlStmt, download, upload, ping, serverID, serverHost, serverLoc, timestamp)
	if err != nil {
		utils.PrintColor("red", "DB insert error:", err.Error())
	}
}

func GetAllSpeedtestResults(w http.ResponseWriter, r *http.Request) {
	dbManager := utils.GetDatabase()
	conn, err := dbManager.GetConnection()
	if err != nil {
		http.Error(w, "DB connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	rows, err := conn.Query("SELECT download, upload, ping, server_id, server_host, server_location, timestamp FROM speedtest_results")
	if err != nil {
		http.Error(w, "DB query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []SpeedtestResult{}
	for rows.Next() {
		var download, upload, pingStr, serverID, serverHost, serverLoc, timestamp string
		if err := rows.Scan(&download, &upload, &pingStr, &serverID, &serverHost, &serverLoc, &timestamp); err != nil {
			utils.PrintColor("red", "Row scan error:", err.Error())
			continue
		}
		results = append(results, SpeedtestResult{
			Download:       download,
			Upload:         upload,
			Ping:           pingStr,
			ServerID:       serverID,
			ServerHost:     serverHost,
			ServerLocation: serverLoc,
			Timestamp:      timestamp,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
