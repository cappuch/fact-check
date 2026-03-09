package main

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DBPath = "factcheck.db"

type FactCheck struct {
	ID        int
	Query     string
	Summary   string
	Sources   string
	Timestamp time.Time
}

func InitDB() error {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS fact_checks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query TEXT NOT NULL,
			summary TEXT NOT NULL,
			sources TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func AddFactCheck(query, summary, sources string) (int64, error) {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	result, err := db.Exec(
		"INSERT INTO fact_checks (query, summary, sources, timestamp) VALUES (?, ?, ?, ?)",
		query, summary, sources, time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func GetFactChecks() ([]FactCheck, error) {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, query, summary, sources, timestamp FROM fact_checks ORDER BY timestamp DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var factChecks []FactCheck
	for rows.Next() {
		var fc FactCheck
		err := rows.Scan(&fc.ID, &fc.Query, &fc.Summary, &fc.Sources, &fc.Timestamp)
		if err != nil {
			return nil, err
		}
		factChecks = append(factChecks, fc)
	}

	return factChecks, rows.Err()
}
