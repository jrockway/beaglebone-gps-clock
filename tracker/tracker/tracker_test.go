package main

import "testing"

func TestSatelliteState(t *testing.T) {
	db, err := OpenDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	c := make(chan satelliteStatus)

	go recordSatellites(c, db)

	c <- satelliteStatus{prn: 1, level: 42, locked: true}
	c <- satelliteStatus{}
	if got, want := trackedSatellites.Value(), `1`; got != want {
		t.Errorf("tracked satellites list:\n  got: %q\n want: %q", got, want)
	}
	n, err := db.single("select count(1) from satellite")
	if err != nil {
		t.Fatalf("count satellites: %v", err)
	}
	if got, want := n, 0; got != want {
		t.Errorf("satellite count after only receiving signal packet:\n  got: %v\n want: %v", got, want)
	}

	c <- satelliteStatus{prn: 1, azimuth: 1, elevation: 1}
	c <- satelliteStatus{}
	if got, want := trackedSatellites.Value(), `1`; got != want {
		t.Errorf("tracked satellites list:\n  got: %q\n want: %q", got, want)
	}
	n, err = db.single("select count(1) from satellite")
	if err != nil {
		t.Fatalf("count satellites: %v", err)
	}
	if got, want := n, 1; got != want {
		t.Errorf("satellite count after both packets:\n  got: %v\n want: %v", got, want)
	}
}
