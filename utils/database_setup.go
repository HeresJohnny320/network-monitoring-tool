package utils

import (
	"database/sql"
	"fmt"
	"strings"
)

func CreateTables(conn *sql.DB, mysql bool) error {
	var pingTable, tracerouteTable, tracerouteHopsTable, speedtestTable string

	if !mysql {
		pingTable = `
		CREATE TABLE IF NOT EXISTS ping_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			host TEXT NOT NULL,
			pass BOOLEAN NOT NULL,
			time_ms INTEGER NOT NULL,
			timestamp DATETIME NOT NULL
		);`
		tracerouteTable = `
		CREATE TABLE IF NOT EXISTS traceroute_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			host TEXT NOT NULL,
			timestamp DATETIME NOT NULL
		);`
		tracerouteHopsTable = `
		CREATE TABLE IF NOT EXISTS traceroute_hops (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			traceroute_id INTEGER NOT NULL,
			hop_number INTEGER NOT NULL,
			ip TEXT,
			time1_ms REAL,
			time2_ms REAL,
			time3_ms REAL,
			FOREIGN KEY(traceroute_id) REFERENCES traceroute_results(id)
		);`
		speedtestTable = `
		CREATE TABLE IF NOT EXISTS speedtest_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			download REAL NOT NULL,
			upload REAL NOT NULL,
			ping REAL NOT NULL,
			server_id INTEGER,
			server_host TEXT,
			server_location TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`
	} else {
		pingTable = `
		CREATE TABLE IF NOT EXISTS ping_results (
			id INT AUTO_INCREMENT PRIMARY KEY,
			host VARCHAR(255) NOT NULL,
			pass TINYINT(1) NOT NULL,
			time_ms INT NOT NULL,
			timestamp DATETIME NOT NULL
		);`
		tracerouteTable = `
		CREATE TABLE IF NOT EXISTS traceroute_results (
			id INT AUTO_INCREMENT PRIMARY KEY,
			host VARCHAR(255) NOT NULL,
			timestamp DATETIME NOT NULL
		);`
		tracerouteHopsTable = `
		CREATE TABLE IF NOT EXISTS traceroute_hops (
			id INT AUTO_INCREMENT PRIMARY KEY,
			traceroute_id INT NOT NULL,
			hop_number INT NOT NULL,
			ip VARCHAR(255),
			time1_ms DOUBLE,
			time2_ms DOUBLE,
			time3_ms DOUBLE,
			FOREIGN KEY(traceroute_id) REFERENCES traceroute_results(id)
		);`
		speedtestTable = `
		CREATE TABLE IF NOT EXISTS speedtest_results (
			id INT AUTO_INCREMENT PRIMARY KEY,
			download DOUBLE NOT NULL,
			upload DOUBLE NOT NULL,
			ping DOUBLE NOT NULL,
			server_id INT,
			server_host VARCHAR(255),
			server_location VARCHAR(255),
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`
	}

	tables := []string{pingTable, tracerouteTable, tracerouteHopsTable, speedtestTable}
	for _, table := range tables {
		_, err := conn.Exec(table)
		if err != nil {
			PrintColor("red", "Failed SQL:\n"+table)
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	if err := migrate(conn, mysql); err != nil {
		return err
	}

	PrintColor("cyan", "Database tables ready.")
	return nil
}

// migrate adds columns introduced after the first release, so existing databases keep working.
func migrate(conn *sql.DB, mysql bool) error {
	real, text := "REAL", "TEXT"
	if mysql {
		real, text = "DOUBLE", "VARCHAR(512)"
	}
	columns := []struct{ table, column, def string }{
		{"ping_results", "loss_pct", real + " DEFAULT 0"},
		{"ping_results", "jitter_ms", real + " DEFAULT 0"},
		{"speedtest_results", "jitter", real},
		{"speedtest_results", "packet_loss", real},
		{"speedtest_results", "isp", text},
		{"speedtest_results", "result_url", text},
	}
	for _, c := range columns {
		if _, err := conn.Exec(fmt.Sprintf("SELECT %s FROM %s WHERE 1=0", c.column, c.table)); err == nil {
			continue
		}
		if _, err := conn.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", c.table, c.column, c.def)); err != nil {
			return fmt.Errorf("failed to add column %s.%s: %v", c.table, c.column, err)
		}
		PrintColor("cyan", "Added column "+c.table+"."+c.column)
	}

	// Every query filters on timestamp, so index it.
	indexes := []struct{ name, table, column string }{
		{"idx_ping_ts", "ping_results", "timestamp"},
		{"idx_speedtest_ts", "speedtest_results", "timestamp"},
		{"idx_traceroute_ts", "traceroute_results", "timestamp"},
		{"idx_hops_traceroute", "traceroute_hops", "traceroute_id"},
	}
	for _, idx := range indexes {
		stmt := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s(%s)", idx.name, idx.table, idx.column)
		if mysql {
			// MySQL has no CREATE INDEX IF NOT EXISTS; "Duplicate key name" just means it exists.
			stmt = fmt.Sprintf("CREATE INDEX %s ON %s(%s)", idx.name, idx.table, idx.column)
		}
		if _, err := conn.Exec(stmt); err != nil && !strings.Contains(err.Error(), "Duplicate key name") {
			return fmt.Errorf("failed to create index %s: %v", idx.name, err)
		}
	}
	return nil
}
