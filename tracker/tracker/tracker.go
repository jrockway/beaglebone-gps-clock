package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jrockway/beaglebone-gps-clock/tracker/trimble"
	"github.com/pkg/term"

	"expvar"
)

var (
	port   = flag.String("port", "", "serial port to read TSIP from; empty to read from gpspipe")
	dbfile = flag.String("db", "tracker.db", "database file to write")
)

var (
	tsipPackets    = expvar.NewInt("tsip_packets")
	tempReadings   = expvar.NewInt("temperature_readings")
	signalReadings = expvar.NewInt("satellite_signal_readings")

	trackedSatellites = expvar.NewString("tracked_satellites")
)

type tempReading struct {
	source string
	value  float64
}

type satelliteStatus struct {
	prn         int
	locked      bool
	level       float32
	azimuth     float32
	elevation   float32
	lastUpdated time.Time
	lastWritten time.Time
}

type summary struct {
	sync.Mutex
	satellites []int
}

func main() {
	flag.Parse()

	db, err := OpenDatabase(*dbfile)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	p := new(trimble.Packetizer)
	p.C = make(chan []byte)
	temp := make(chan tempReading)
	sats := make(chan satelliteStatus)

	go recordTemperatures(temp, db)
	go recordSatellites(sats, db)
	go readRTCTemperature(temp)
	go readTSIP(p)
	go readGPSStatus(p.C, temp, sats)

	log.Fatal(http.ListenAndServe(":8888", nil))
}

func readTSIP(p *trimble.Packetizer) {
	if *port == "" {
		log.Printf("reading from gpspipe")
		cmd := exec.Command("gpspipe", "-R")
		cmd.Stdout = p

		go func() {
			if err := cmd.Run(); err != nil {
				log.Fatalf("running gpspipe: %v", err)
			}
		}()

	} else {
		port, err := term.Open(*port, term.Speed(9600), term.RawMode)
		if err != nil {
			log.Fatal(err)
		}

		for {
			if _, err := io.Copy(p, port); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func readGPSStatus(packets chan []byte, temp chan tempReading, sats chan satelliteStatus) {
	for packet := range packets {
		tsipPackets.Add(1)
		p, err := trimble.ParsePacket(packet)
		if err != nil {
			log.Printf("parse error: %q: %v\n", base64.StdEncoding.EncodeToString(packet), err)
		}

		if p.SupplementalTiming != nil {
			temp <- tempReading{source: "GPS", value: float64(p.SupplementalTiming.Temperature)}
		}

		if p.RawMeasurement != nil {
			sats <- satelliteStatus{
				prn:    p.RawMeasurement.PRN,
				level:  p.RawMeasurement.SignalLevel.Level(),
				locked: p.RawMeasurement.SignalLevel.Locked(),
			}
		}
	}
}

func recordTemperatures(c chan tempReading, db *DB) {
	last := make(map[string]time.Time)
	for r := range c {
		tempReadings.Add(1)
		if time.Since(last[r.source]) > time.Minute {
			if err := db.RecordTemperature(r.source, r.value); err != nil {
				log.Printf("error logging temperature: %v", err)
				continue
			}

			last[r.source] = time.Now()
		}
	}
}

func readRTCTemperature(c chan tempReading) {
	read := func() {
		bytes, err := ioutil.ReadFile("/sys/class/rtc/rtc0/device/hwmon/hwmon0/temp1_input")
		if err != nil {
			log.Printf("error reading rtc temperature: %v", err)
			return
		}
		str := strings.TrimSpace(string(bytes))

		t, err := strconv.Atoi(str)
		if err != nil {
			log.Printf("error parsing rtc temperature: %q %v", str, err)
			return
		}

		c <- tempReading{source: "RTC", value: float64(t) / 1000}
	}

	read()
	for _ = range time.Tick(5 * time.Minute) {
		read()
	}
}

func recordSatellites(c chan satelliteStatus, db *DB) {
	sats := make(map[int]*satelliteStatus)
	for reading := range c {
		signalReadings.Add(1)
		if _, ok := sats[reading.prn]; !ok {
			sats[reading.prn] = new(satelliteStatus)
		}

		state := sats[reading.prn]
		state.prn = reading.prn
		if reading.level != 0 {
			state.level = reading.level
			state.locked = reading.locked
		}

		if reading.azimuth != 0 || reading.elevation != 0 {
			state.azimuth = reading.azimuth
			state.elevation = reading.elevation
		}

		now := time.Now()
		state.lastUpdated = now

		if state.level != 0 && (state.azimuth != 0 || state.elevation != 0) && time.Since(state.lastWritten) > 5*time.Minute {
			if err := db.RecordSatelliteStatus(state.prn, state.level, state.azimuth, state.elevation); err != nil {
				log.Printf("error writing satellite status to database: %v", err)
			} else {
				state.lastWritten = now
			}
		}

		var tracked []string
		for _, state := range sats {
			if state.locked && state.level > 0 && time.Since(state.lastUpdated) < time.Minute {
				tracked = append(tracked, fmt.Sprintf("%d", state.prn))
			}
		}
		trackedSatellites.Set(strings.Join(tracked, ","))
	}
}
