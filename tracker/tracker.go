package main

import (
	"encoding/base64"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
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
	tsipPackets  = expvar.NewInt("tsip_packets")
	tempReadings = expvar.NewInt("temperature_readings")
)

type tempReading struct {
	source string
	value  float64
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

	go recordTemperatures(temp, db)
	go readRTCTemperature(temp)
	go readTSIP(p)
	go readGPSStatus(p.C, temp)

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

func readGPSStatus(packets chan []byte, temp chan tempReading) {
	for packet := range packets {
		tsipPackets.Add(1)
		p, err := trimble.ParsePacket(packet)
		if err != nil {
			log.Printf("parse error: %q: %v\n", base64.StdEncoding.EncodeToString(packet), err)
		}

		if p.SupplementalTiming != nil {
			temp <- tempReading{source: "GPS", value: float64(p.SupplementalTiming.Temperature)}
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
