package tools

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"network_monitor_tool/utils"
)

type PingResult struct {
	Host     string
	Sent     int
	Received int
	AvgMs    float64
	MinMs    float64
	MaxMs    float64
	JitterMs float64
	LossPct  float64
}

func (p PingResult) Pass() bool { return p.Received > 0 }

// PingHosts pings every host in parallel and stores the results.
func PingHosts(hosts []string, count int) []PingResult {
	results := make([]PingResult, len(hosts))
	var wg sync.WaitGroup
	for i, host := range hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = PingHost(host, count)
		}()
	}
	wg.Wait()

	timestamp := utils.DBTime(time.Now())
	for _, r := range results {
		utils.PrintColor("blue", fmt.Sprintf("Ping %s: %d/%d replies, avg %.1fms, jitter %.1fms, loss %.0f%%",
			r.Host, r.Received, r.Sent, r.AvgMs, r.JitterMs, r.LossPct))
		if err := savePing(r, timestamp); err != nil {
			utils.PrintColor("red", "DB insert error: "+err.Error())
		}
	}
	return results
}

func savePing(r PingResult, timestamp string) error {
	conn, err := db()
	if err != nil {
		return err
	}
	// Stored unrounded: SQLite keeps the fraction (handy for sub-ms LAN hosts),
	// MySQL's INT column rounds it.
	timeMs := -1.0
	if r.Pass() {
		timeMs = r.AvgMs
	}
	_, err = conn.Exec(`INSERT INTO ping_results (host, pass, time_ms, loss_pct, jitter_ms, timestamp) VALUES (?, ?, ?, ?, ?, ?)`,
		r.Host, boolToInt(r.Pass()), timeMs, r.LossPct, r.JitterMs, timestamp)
	return err
}

func PingHost(host string, count int) PingResult {
	if count < 1 {
		count = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(count*3+10)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", strconv.Itoa(count), host)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", strconv.Itoa(count), host)
	}
	// ping exits non-zero when packets are lost, but the output is still worth parsing.
	output, _ := cmd.Output()
	return ParsePingOutput(host, count, string(output))
}

// Matches the round trip time in a reply line on every OS and locale we've seen:
// "time=12.3 ms" (Linux/macOS/BSD), "time=12ms" / "time<1ms" / "Zeit=12ms" (Windows).
var replyTimeRe = regexp.MustCompile(`[=<]\s*(\d+(?:[.,]\d+)?)\s*ms`)

// ParsePingOutput counts reply lines (they all contain "ttl=", in any language) and
// derives latency, jitter and packet loss from them.
func ParsePingOutput(host string, sent int, output string) PingResult {
	result := PingResult{Host: host, Sent: sent, AvgMs: -1}
	var times []float64
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(strings.ToLower(line), "ttl=") {
			continue
		}
		m := replyTimeRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
		if err != nil {
			continue
		}
		times = append(times, v)
	}
	if len(times) > sent {
		times = times[:sent] // ignore duplicate replies
	}

	result.Received = len(times)
	if sent > 0 {
		result.LossPct = float64(sent-result.Received) / float64(sent) * 100
	}
	if len(times) == 0 {
		return result
	}

	sum, minV, maxV := 0.0, times[0], times[0]
	for _, t := range times {
		sum += t
		minV = math.Min(minV, t)
		maxV = math.Max(maxV, t)
	}
	result.AvgMs = sum / float64(len(times))
	result.MinMs = minV
	result.MaxMs = maxV
	if len(times) > 1 {
		diff := 0.0
		for i := 1; i < len(times); i++ {
			diff += math.Abs(times[i] - times[i-1])
		}
		result.JitterMs = diff / float64(len(times)-1)
	}
	return result
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type PingRow struct {
	ID        int64   `json:"id"`
	Host      string  `json:"host"`
	Success   bool    `json:"success"`
	TimeMs    float64 `json:"time_ms"`
	LossPct   float64 `json:"loss_pct"`
	JitterMs  float64 `json:"jitter_ms"`
	Timestamp string  `json:"timestamp"`
}

func QueryPings(r TimeRange, host string) ([]PingRow, error) {
	conn, err := db()
	if err != nil {
		return nil, err
	}
	where, args := r.where("timestamp")
	if host != "" {
		if where == "" {
			where = " WHERE host = ?"
		} else {
			where += " AND host = ?"
		}
		args = append(args, host)
	}
	rows, err := conn.Query("SELECT id, host, pass, time_ms, loss_pct, jitter_ms, timestamp FROM ping_results"+where+" ORDER BY timestamp, id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []PingRow{}
	for rows.Next() {
		var p PingRow
		var loss, jitter sql.NullFloat64
		var ts any
		if err := rows.Scan(&p.ID, &p.Host, &p.Success, &p.TimeMs, &loss, &jitter, &ts); err != nil {
			utils.PrintColor("red", "Row scan error: "+err.Error())
			continue
		}
		p.LossPct = nullFloat(loss)
		p.JitterMs = nullFloat(jitter)
		if !p.Success && p.LossPct == 0 {
			p.LossPct = 100 // rows from before loss was recorded
		}
		p.Timestamp = utils.FormatDBTime(ts)
		results = append(results, p)
	}
	return results, rows.Err()
}
