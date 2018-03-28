package main

import "testing"

func TestDatabase(t *testing.T) {
	db, err := OpenDatabase(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}

	if err := db.RecordTemperature("foo", 42.1234); err != nil {
		t.Errorf("record temperature: %v", err)
	}

	if err := db.RecordSatelliteStatus(1, 30, 1, 2, 3); err != nil {
		t.Errorf("record satellite status: %v", err)
	}
}
