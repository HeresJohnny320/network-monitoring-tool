package utils

import (
	"database/sql"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

// One shared connection pool for the whole app (it used to open a new pool, and
// ping it, for every single query).
var (
	dbMu      sync.RWMutex
	dbConn    *sql.DB
	dbIsMySQL bool
)

// DB returns the shared connection pool.
func DB() *sql.DB {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return dbConn
}

func IsMySQL() bool {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return dbIsMySQL
}

// DatabaseDescription is a human readable "where is my data" string for the UI.
func DatabaseDescription(c Config) string {
	if c.RunSQL {
		return fmt.Sprintf("MySQL %s@%s/%s", c.SQLUser, net.JoinHostPort(c.SQLHost, c.SQLPort), c.SQLDatabase)
	}
	dir, _ := DataDir()
	return "SQLite " + filepath.Join(dir, "database.db")
}

func openDatabase(c Config) (*sql.DB, error) {
	var conn *sql.DB
	var err error
	if !c.RunSQL {
		dir, derr := DataDir()
		if derr != nil {
			return nil, fmt.Errorf("cannot get data dir: %v", derr)
		}
		path := filepath.ToSlash(filepath.Join(dir, "database.db"))
		conn, err = sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
		if err != nil {
			return nil, err
		}
		// SQLite only allows one writer anyway; a single connection avoids "database is locked".
		conn.SetMaxOpenConns(1)
	} else {
		mc := mysql.NewConfig()
		mc.User = c.SQLUser
		mc.Passwd = c.SQLPassword
		mc.Net = "tcp"
		port := c.SQLPort
		if port == "" {
			port = "3306"
		}
		mc.Addr = net.JoinHostPort(c.SQLHost, port)
		mc.DBName = c.SQLDatabase
		mc.Timeout = 10 * time.Second
		mc.ReadTimeout = 30 * time.Second
		mc.WriteTimeout = 30 * time.Second
		conn, err = sql.Open("mysql", mc.FormatDSN())
		if err != nil {
			return nil, err
		}
		conn.SetMaxOpenConns(5)
		conn.SetConnMaxLifetime(5 * time.Minute)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("cannot connect to database: %v", err)
	}
	if err := CreateTables(conn, c.RunSQL); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// InitDatabase opens (or switches to) the database described by c and makes sure
// all tables exist. The old pool is only replaced once the new one works.
func InitDatabase(c Config) error {
	conn, err := openDatabase(c)
	if err != nil {
		return err
	}
	dbMu.Lock()
	old := dbConn
	dbConn = conn
	dbIsMySQL = c.RunSQL
	dbMu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

func CloseDatabase() {
	dbMu.Lock()
	defer dbMu.Unlock()
	if dbConn != nil {
		dbConn.Close()
		dbConn = nil
	}
}

// Timestamps are stored as UTC text in this layout so string comparison sorts them.
const DBTimeLayout = "2006-01-02 15:04:05"

func DBTime(t time.Time) string {
	return t.UTC().Format(DBTimeLayout)
}

var dbTimeLayouts = []string{
	DBTimeLayout,
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05",
}

// ParseDBTime turns whatever the driver hands back for a timestamp column
// (time.Time from SQLite, []byte from MySQL, or text) into a UTC time.
func ParseDBTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t.UTC(), true
	case []byte:
		return ParseDBTime(string(t))
	case string:
		s := strings.TrimSpace(t)
		for _, layout := range dbTimeLayouts {
			if parsed, err := time.Parse(layout, s); err == nil {
				return parsed.UTC(), true
			}
		}
	}
	return time.Time{}, false
}

// FormatDBTime returns an RFC3339 UTC timestamp for the API so browsers convert it to
// local time correctly (the old "2006-01-02 15:04:05" got parsed as local time).
func FormatDBTime(v any) string {
	if t, ok := ParseDBTime(v); ok {
		return t.Format(time.RFC3339)
	}
	switch t := v.(type) {
	case []byte:
		return string(t)
	case string:
		return t
	}
	return ""
}
