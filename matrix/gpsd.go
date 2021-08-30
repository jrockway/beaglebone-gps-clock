package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/stratoberry/go-gpsd"
	"golang.org/x/net/trace"
)

func watchGpsd() {
	l := trace.NewEventLog("service", "gpsd")
	defer l.Finish()
	for {
		monitorGpsd(l)
		time.Sleep(10 * time.Second)
	}
}

func monitorGpsd(l trace.EventLog) {
	watchdog := make(chan struct{})
	l.Printf("dial localhost:2947")
	gps, err := gpsd.Dial("localhost:2947")
	if err != nil {
		l.Errorf("dial gpsd: %v", err)
		return
	}
	gps.AddFilter("SKY", func(r interface{}) {
		select {
		case watchdog <- struct{}{}:
		default:
		}
		t := time.Now()
		sky := r.(*gpsd.SKYReport)
		buf := new(strings.Builder)
		l.Printf("sky report: %#v", sky)
		for _, s := range sky.Satellites {
			used := "0u"
			if s.Used {
				used = "1u"
			}
			buf.WriteString(fmt.Sprintf("satellite,prn=%v azimuth=%v,elevation=%v,snr=%v,used=%v %v\n", s.PRN, s.Az, s.El, s.Ss, used, t.UnixNano()))
		}
		buf.WriteString(fmt.Sprintf("dop xdop=%v,ydop=%v,vdop=%v,tdop=%v,hdop=%v,pdop=%v,gdop=%v %v\n", sky.Xdop, sky.Ydop, sky.Vdop, sky.Tdop, sky.Hdop, sky.Pdop, sky.Gdop, t.UnixNano()))
		if err := sendToInflux(buf.String()); err != nil {
			l.Errorf("write satellite status to influx: %v", err)
			return
		}
	})
	log.Printf("starting gpsd watch loop")
	for {
		select {
		case <-gps.Watch():
			l.Errorf("gpsd watch stopped; restarting")
			return
		case <-time.After(time.Minute):
			l.Errorf("gpsd hasn't sent data for 1 minute; restarting")
			return
		case <-watchdog:
			continue
		}
	}
}
