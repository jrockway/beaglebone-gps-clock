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

	c, err := db.single("select count(1) from temperature")
	if err != nil {
		t.Fatalf("count temperature: %v", err)
	}
	if got, want := c, 1; got != want {
		t.Errorf("unexpected number of temperature rows:\n  got: %d\n want: %d", got, want)
	}

	if err := db.RecordSatelliteStatus(1, 30, 1, 2); err != nil {
		t.Errorf("record satellite status: %v", err)
	}

	c, err = db.single("select count(1) from satellite")
	if err != nil {
		t.Fatalf("count satellites: %v", err)
	}
	if got, want := c, 1; got != want {
		t.Errorf("unexpected number of satellite rows:\n  got: %d\n want: %d", got, want)
	}
}
