package tools

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"network_monitor_tool/utils"
)

type TracerouteHop struct {
	Hop   int        `json:"hop"`
	IP    string     `json:"ip"`
	Times []*float64 `json:"times"`
}

type TracerouteResult struct {
	ID        int64           `json:"id,omitempty"`
	Host      string          `json:"host"`
	Timestamp string          `json:"timestamp"`
	Hops      []TracerouteHop `json:"hops"`
}

// RunTracerouteForHosts traces every host in parallel and returns the first error.
func RunTracerouteForHosts(hosts []string) error {
	errs := make([]error, len(hosts))
	var wg sync.WaitGroup
	for i, host := range hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = RunTraceroute(host)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func RunTraceroute(host string) (TracerouteResult, error) {
	now := time.Now()
	result := TracerouteResult{
		Host:      host,
		Timestamp: now.UTC().Format(time.RFC3339),
		Hops:      []TracerouteHop{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "tracert", "-d", "-h", "30", "-w", "1000", host)
	} else {
		cmd = exec.CommandContext(ctx, "traceroute", "-n", "-m", "30", "-w", "2", "-q", "3", host)
	}

	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		utils.PrintColor("red", "Traceroute error ("+host+"): "+err.Error())
		return result, fmt.Errorf("traceroute %s: %v", host, err)
	}
	result.Hops = ParseTracerouteOutput(string(output))
	utils.PrintColor("magenta", fmt.Sprintf("Traceroute %s: %d hops", host, len(result.Hops)))

	id, err := saveTraceroute(result, utils.DBTime(now))
	if err != nil {
		utils.PrintColor("red", "DB insert error: "+err.Error())
		return result, err
	}
	result.ID = id
	return result, nil
}

func saveTraceroute(result TracerouteResult, timestamp string) (int64, error) {
	conn, err := db()
	if err != nil {
		return 0, err
	}
	tx, err := conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO traceroute_results (host, timestamp) VALUES (?, ?)`, result.Host, timestamp)
	if err != nil {
		return 0, err
	}
	tracerouteID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	hopStmt, err := tx.Prepare(`INSERT INTO traceroute_hops (traceroute_id, hop_number, ip, time1_ms, time2_ms, time3_ms) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer hopStmt.Close()

	for _, h := range result.Hops {
		times := [3]*float64{}
		for i := 0; i < 3 && i < len(h.Times); i++ {
			times[i] = h.Times[i]
		}
		if _, err := hopStmt.Exec(tracerouteID, h.Hop, h.IP, times[0], times[1], times[2]); err != nil {
			return 0, err
		}
	}
	return tracerouteID, tx.Commit()
}

var hopLineRe = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)

// ParseTracerouteOutput handles traceroute (Linux/macOS/BSD) and tracert (Windows):
//
//	3  10.0.0.1  5.123 ms  4.987 ms *
//	3     5 ms    <1 ms     4 ms  10.0.0.1
//	4     *        *        *     Request timed out.
func ParseTracerouteOutput(output string) []TracerouteHop {
	hops := []TracerouteHop{}
	for _, line := range strings.Split(output, "\n") {
		m := hopLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		hopNumber, _ := strconv.Atoi(m[1])
		hop := TracerouteHop{Hop: hopNumber, IP: "*", Times: []*float64{}}

		fields := strings.Fields(m[2])
		for i := 0; i < len(fields); i++ {
			f := fields[i]
			switch {
			case f == "*":
				hop.Times = append(hop.Times, nil)
			case strings.HasSuffix(f, "ms") && len(f) > 2: // Windows "<1ms" or "12ms"
				if v, ok := parseMs(strings.TrimSuffix(f, "ms")); ok {
					hop.Times = append(hop.Times, &v)
				}
			case i+1 < len(fields) && fields[i+1] == "ms":
				if v, ok := parseMs(f); ok {
					hop.Times = append(hop.Times, &v)
				}
				i++
			default:
				if ip := strings.Trim(f, "()[]"); hop.IP == "*" && net.ParseIP(ip) != nil {
					hop.IP = ip
				}
			}
		}
		if len(hop.Times) > 3 {
			hop.Times = hop.Times[:3]
		}
		for len(hop.Times) < 3 {
			hop.Times = append(hop.Times, nil)
		}
		hops = append(hops, hop)
	}
	return hops
}

func parseMs(s string) (float64, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "<")
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// QueryTraceroutes returns traceroutes in the range, newest first. limit <= 0 means no limit.
func QueryTraceroutes(r TimeRange, limit int) ([]TracerouteResult, error) {
	conn, err := db()
	if err != nil {
		return nil, err
	}
	where, args := r.where("timestamp")
	query := "SELECT id, host, timestamp FROM traceroute_results" + where + " ORDER BY id DESC"
	if limit > 0 {
		query += " LIMIT " + strconv.Itoa(limit)
	}
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}

	results := []TracerouteResult{}
	index := map[int64]int{}
	for rows.Next() {
		var res TracerouteResult
		var ts any
		if err := rows.Scan(&res.ID, &res.Host, &ts); err != nil {
			continue
		}
		res.Timestamp = utils.FormatDBTime(ts)
		res.Hops = []TracerouteHop{}
		index[res.ID] = len(results)
		results = append(results, res)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load hops in batches instead of one query per traceroute.
	const batch = 500
	for start := 0; start < len(results); start += batch {
		end := min(start+batch, len(results))
		ids := make([]any, 0, end-start)
		for _, res := range results[start:end] {
			ids = append(ids, res.ID)
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		hopRows, err := conn.Query(`SELECT traceroute_id, hop_number, ip, time1_ms, time2_ms, time3_ms FROM traceroute_hops
			WHERE traceroute_id IN (`+placeholders+`) ORDER BY traceroute_id, hop_number`, ids...)
		if err != nil {
			return nil, err
		}
		for hopRows.Next() {
			var id int64
			var h TracerouteHop
			var ip sql.NullString
			var t1, t2, t3 sql.NullFloat64
			if err := hopRows.Scan(&id, &h.Hop, &ip, &t1, &t2, &t3); err != nil {
				continue
			}
			h.IP = ip.String
			for _, t := range []sql.NullFloat64{t1, t2, t3} {
				if t.Valid {
					val := t.Float64
					h.Times = append(h.Times, &val)
				} else {
					h.Times = append(h.Times, nil)
				}
			}
			if i, ok := index[id]; ok {
				results[i].Hops = append(results[i].Hops, h)
			}
		}
		hopRows.Close()
	}
	return results, nil
}
