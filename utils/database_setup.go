package utils

import (
	"fmt"
)

func CreateTables() error {
	dbManager := GetDatabase()
	conn, err := dbManager.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to get DB connection: %v", err)
	}
	defer conn.Close()

	var pingTable, tracerouteTable, tracerouteHopsTable, speedtestTable string

	if !dbManager.RunSQL {
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

	PrintColor("cyan", "Database tables created (if not exist).")
	return nil
}
