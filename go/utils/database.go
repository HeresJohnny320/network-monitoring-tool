package utils

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
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
	}

	db := &DBManager{
		RunSQL: Cfg.RunSQL,
	}

	if !db.RunSQL {
		db.URL = "database.db"
		db.User = ""
		db.Password = ""
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
	if !db.RunSQL {
		return sql.Open("sqlite3", db.URL)
	}
	return sql.Open("mysql", db.URL)
}
