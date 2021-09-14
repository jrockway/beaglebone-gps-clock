package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jrockway/go-gpsd"
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
	gps.AddFilter("TPV", func(r interface{}) {
		t := time.Now()
		tpv, ok := r.(*gpsd.TPVReport)
		if !ok {
			l.Errorf("not a TPVReport: %#v", r)
		}
		select {
		case watchdog <- struct{}{}:
		default:
		}
		l.Printf("tpv report: %#v", tpv)
		AddPosition(tpv.Device, tpv.Lat, tpv.Lon, tpv.Alt)
		msg := fmt.Sprintf("tpv,device=%v lat=%v,lon=%v,alt=%v %v\n", tpv.Device, tpv.Lat, tpv.Lon, tpv.Alt, t.UnixNano())
		if err := sendToInflux(msg); err != nil {
			l.Errorf("write tpv report to influx: %v", err)
			return
		}
	})
	gps.AddFilter("SKY", func(r interface{}) {
		t := time.Now()
		sky, ok := r.(*gpsd.SKYReport)
		if !ok {
			l.Errorf("not a SKYReport: %#v", r)
			return
		}
		select {
		case watchdog <- struct{}{}:
		default:
		}
		l.Printf("sky report: %#v", sky)
		sort.Slice(sky.Satellites, func(i, j int) bool {
			return sky.Satellites[i].PRN < sky.Satellites[j].PRN
		})

		buf := new(strings.Builder)
		for _, s := range sky.Satellites {
			used := "0u"
			if s.Used {
				used = "1u"
			}
			buf.WriteString(fmt.Sprintf("satellite,device=%v,prn=%v azimuth=%v,elevation=%v,snr=%v,used=%v %v\n", sky.Device, s.PRN, s.Az, s.El, s.Ss, used, t.UnixNano()))
			AddSatellite(sky.Device, s)
		}
		buf.WriteString(fmt.Sprintf("dop,device=%v xdop=%v,ydop=%v,vdop=%v,tdop=%v,hdop=%v,pdop=%v,gdop=%v %v\n", sky.Device, sky.Xdop, sky.Ydop, sky.Vdop, sky.Tdop, sky.Hdop, sky.Pdop, sky.Gdop, t.UnixNano()))
		if err := sendToInflux(buf.String()); err != nil {
			l.Errorf("write satellite status to influx: %v", err)
			return
		}
	})
	log.Printf("starting gpsd watch loop")
	watchCh := gps.Watch()
	for {
		select {
		case <-watchCh:
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
