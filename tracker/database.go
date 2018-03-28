package main

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const initDatabase = `
CREATE TABLE IF NOT EXISTS temperature (date datetime not null, source text not null, temperature double not null);
CREATE TABLE IF NOT EXISTS satellite (date datetime not null, prn integer not null, strength double, azimuth double, elevation double);
`

type DB struct {
	*sql.DB
}

func OpenDatabase(filename string) (*DB, error) {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(initDatabase); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) RecordTemperature(source string, temperature float64) error {
	s, err := db.Prepare("insert into temperature values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer s.Close()
	if _, err := s.Exec(time.Now(), source, temperature); err != nil {
		return err
	}
	return nil
}

func (db *DB) RecordSatelliteStatus(prn int, strength, azimuth, elevation float64) error {
	s, err := db.Prepare("insert into satellite values(?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	s.Close()
	if _, err := s.Exec(time.Now(), prn, strength, azimuth, elevation); err != nil {
		return err
	}
	return nil
}
