package tools

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"network_monitor_tool/utils"
)

// TimeRange filters results by timestamp. Zero From/To means unbounded.
type TimeRange struct {
	From time.Time
	To   time.Time
}

func (r TimeRange) where(column string) (string, []any) {
	var conds []string
	var args []any
	if !r.From.IsZero() {
		conds = append(conds, column+" >= ?")
		args = append(args, utils.DBTime(r.From))
	}
	if !r.To.IsZero() {
		conds = append(conds, column+" <= ?")
		args = append(args, utils.DBTime(r.To))
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

var errNoDatabase = errors.New("database not initialized")

func db() (*sql.DB, error) {
	conn := utils.DB()
	if conn == nil {
		return nil, errNoDatabase
	}
	return conn, nil
}

func nullFloat(v sql.NullFloat64) float64 {
	if v.Valid {
		return v.Float64
	}
	return 0
}

// Prune deletes results older than the retention period.
func Prune(retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	conn, err := db()
	if err != nil {
		return
	}
	cutoff := utils.DBTime(time.Now().AddDate(0, 0, -retentionDays))
	stmts := []string{
		"DELETE FROM ping_results WHERE timestamp < ?",
		"DELETE FROM speedtest_results WHERE timestamp < ?",
		"DELETE FROM traceroute_hops WHERE traceroute_id IN (SELECT id FROM traceroute_results WHERE timestamp < ?)",
		"DELETE FROM traceroute_results WHERE timestamp < ?",
	}
	var removed int64
	for _, s := range stmts {
		res, err := conn.Exec(s, cutoff)
		if err != nil {
			utils.PrintColor("red", "Prune error: "+err.Error())
			return
		}
		n, _ := res.RowsAffected()
		removed += n
	}
	if removed > 0 {
		utils.PrintColor("cyan", "Removed old results older than "+cutoff+" UTC")
	}
}
