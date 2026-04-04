package utils

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	// _ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

type DBManager struct {
	RunSQL   bool
	URL      string
	User     string
	Password string
}

var instance *DBManager

func InitDatabase() *DBManager {
	if instance != nil {
		return instance
	}

	if Cfg == nil {
		PrintColor("red", "config not loaded. Call LoadConfig() first.")
		return nil
	}

	db := &DBManager{
		RunSQL: Cfg.RunSQL,
	}

	if !db.RunSQL {
		configDir, err := os.UserConfigDir()
		if err != nil {
			PrintColor("red", "cannot get user config dir: "+err.Error())
			return nil
		}
		appDir := filepath.Join(configDir, "network_monitor_tool")

		if err := os.MkdirAll(appDir, 0755); err != nil {
			PrintColor("red", "cannot create app config dir: "+err.Error())
			return nil
		}

		db.URL = filepath.ToSlash(filepath.Join(appDir, "database.db"))
		db.User = ""
		db.Password = ""

		if _, err := os.Stat(db.URL); os.IsNotExist(err) {
			file, err := os.Create(db.URL)
			if err != nil {
				PrintColor("red", "cannot create SQLite file: "+err.Error())
				return nil
			}
			PrintColor("bright_green", "created database.db at "+appDir)
			file.Close()
		}
		f, err := os.OpenFile(db.URL, os.O_RDWR, 0666)
		if err != nil {
			PrintColor("red", "cannot write to SQLite file: "+err.Error())
			return nil
		}
		f.Close()
	} else {
		user := Cfg.SQLUser
		password := Cfg.SQLPassword
		host := Cfg.SQLHost
		port := Cfg.SQLPort
		database := Cfg.SQLDatabase
		db.URL = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, database)
		db.User = user
		db.Password = password
	}

	instance = db
	return instance
}

func GetDatabase() *DBManager {
	if instance == nil {
		PrintColor("red", "database not initialized. Call InitDatabase() first.")
	}
	return instance
}

func (db *DBManager) GetConnection() (*sql.DB, error) {
	var conn *sql.DB
	var err error
	if !db.RunSQL {
		conn, err = sql.Open("sqlite", db.URL)
	} else {
		conn, err = sql.Open("mysql", db.URL)
	}
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("cannot ping database: %v", err)
	}

	return conn, nil
}
