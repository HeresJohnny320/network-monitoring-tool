package tools

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"network_monitor_tool/utils"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type TracerouteHop struct {
	Hop   int        `json:"hop"`
	IP    string     `json:"ip"`
	Times []*float64 `json:"times"`
}

type TracerouteResult struct {
	ID        int             `json:"id,omitempty"`
	Host      string          `json:"host"`
	Timestamp string          `json:"timestamp"`
	Hops      []TracerouteHop `json:"hops"`
}

func RunTracerouteForHosts(hosts []string) {
	if len(hosts) == 0 {
		return
	}
	for _, host := range hosts {
		RunTraceroute(host)
	}
}

func RunTraceroute(host string) TracerouteResult {
	result := TracerouteResult{
		Host:      host,
		Timestamp: time.Now().Format(time.RFC3339),
		Hops:      []TracerouteHop{},
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("tracert", "-d", host)
	} else {
		cmd = exec.Command("traceroute", "-n", host)
	}

	output, err := cmd.Output()
	if err != nil {
		utils.PrintColor("red", "Traceroute error:", err.Error())
		return result
	}

	lines := strings.Split(string(output), "\n")
	hopNumber := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		hop, ok := parseHop(line, runtime.GOOS, hopNumber+1)
		if ok {
			result.Hops = append(result.Hops, hop)
			hopNumber++
		}
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		utils.PrintColor("red", "JSON marshal error:", err.Error())
	} else {
		utils.PrintColor("magenta", "Traceroute result:"+string(jsonBytes))
	}

	db := utils.GetDatabase()
	conn, err := db.GetConnection()
	if err != nil {
		utils.PrintColor("red", "DB connection error:", err.Error())
		return result
	}
	defer conn.Close()

	resStmt, err := conn.Prepare(`INSERT INTO traceroute_results (host, timestamp) VALUES (?, ?)`)
	if err != nil {
		utils.PrintColor("red", "DB prepare error:", err.Error())
		return result
	}
	defer resStmt.Close()

	res, err := resStmt.Exec(result.Host, result.Timestamp)
	if err != nil {
		utils.PrintColor("red", "DB insert error:", err.Error())
		return result
	}

	tracerouteID, err := res.LastInsertId()
	if err != nil {
		utils.PrintColor("red", "DB last insert id error:", err.Error())
		return result
	}

	hopStmt, err := conn.Prepare(`INSERT INTO traceroute_hops (traceroute_id, hop_number, ip, time1_ms, time2_ms, time3_ms) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		utils.PrintColor("red", "DB prepare hop error:", err.Error())
		return result
	}
	defer hopStmt.Close()

	for _, h := range result.Hops {
		times := [3]*float64{}
		for i := 0; i < 3 && i < len(h.Times); i++ {
			times[i] = h.Times[i]
		}
		_, _ = hopStmt.Exec(tracerouteID, h.Hop, h.IP, times[0], times[1], times[2])
	}

	return result
}

func parseHop(line, osName string, hopNumber int) (TracerouteHop, bool) {
	hop := TracerouteHop{
		Hop:   hopNumber,
		Times: []*float64{},
	}

	if strings.Contains(osName, "win") {
		pattern := regexp.MustCompile(`^\s*(\d+)\s+(\*|\d+\s*ms|\*\s*)\s+(\*|\d+\s*ms|\*\s*)\s+(\*|\d+\s*ms|\*\s*)\s+([\d\.]+|\*)$`)
		matches := pattern.FindStringSubmatch(line)
		if len(matches) == 0 {
			return hop, false
		}

		hop.IP = matches[5]
		timeFields := []string{matches[2], matches[3], matches[4]}
		for _, t := range timeFields {
			hop.Times = append(hop.Times, parseTime(strings.TrimSpace(t)))
		}
		return hop, true

	} else {
		pattern := regexp.MustCompile(`^\s*(\d+)\s+([\d\.]+|\*)\s+(.*)$`)
		matches := pattern.FindStringSubmatch(line)
		if len(matches) == 0 {
			return hop, false
		}

		hop.IP = matches[2]
		rest := strings.TrimSpace(matches[3])

		timePattern := regexp.MustCompile(`(\d+\.\d+\s*ms|\*)`)
		timeMatches := timePattern.FindAllString(rest, -1)
		for _, t := range timeMatches {
			hop.Times = append(hop.Times, parseTime(t))
		}

		for len(hop.Times) < 3 {
			hop.Times = append(hop.Times, nil)
		}

		return hop, true
	}
}

func parseTime(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "*" || s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "ms")
	s = strings.TrimSpace(s)
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &val
}

func GetAllTraceroutes(w http.ResponseWriter, r *http.Request) {
	db := utils.GetDatabase()
	conn, err := db.GetConnection()
	if err != nil {
		http.Error(w, "DB connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	results := []TracerouteResult{}
	rows, err := conn.Query(`SELECT id, host, timestamp FROM traceroute_results ORDER BY id DESC`)
	if err != nil {
		http.Error(w, "DB query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var res TracerouteResult
		var tracerouteID int
		if err := rows.Scan(&tracerouteID, &res.Host, &res.Timestamp); err != nil {
			continue
		}
		res.ID = tracerouteID

		hopsRows, err := conn.Query(`SELECT hop_number, ip, time1_ms, time2_ms, time3_ms FROM traceroute_hops WHERE traceroute_id = ? ORDER BY hop_number ASC`, tracerouteID)
		if err != nil {
			continue
		}
		for hopsRows.Next() {
			var h TracerouteHop
			var t1, t2, t3 sql.NullFloat64
			if err := hopsRows.Scan(&h.Hop, &h.IP, &t1, &t2, &t3); err != nil {
				continue
			}
			times := []*float64{}
			for _, t := range []sql.NullFloat64{t1, t2, t3} {
				if t.Valid {
					val := t.Float64
					times = append(times, &val)
				} else {
					times = append(times, nil)
				}
			}
			h.Times = times
			res.Hops = append(res.Hops, h)
		}
		hopsRows.Close()

		results = append(results, res)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
