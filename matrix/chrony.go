package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/facebookincubator/ntp/protocol/chrony"
	"golang.org/x/net/trace"
)

const source = "beaglebone"

func watchChrony() {
	l := trace.NewEventLog("service", "chrony")
	defer l.Finish()
	for {
		if err := monitorChrony(l); err != nil {
			log.Printf("monitorChrony exited unexpectedly: %v", err)
			l.Errorf("monitorChrony exited unexpectedly: %v", err)
			time.Sleep(10 * time.Second)
		}
	}
}

func monitorChrony(l trace.EventLog) error {
	l.Printf("dial localhost:323")
	conn, err := net.DialTimeout("udp", "localhost:323", time.Second)
	if err != nil {
		l.Errorf("dial: %v", err)
		return fmt.Errorf("dial: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Minute)); err != nil {
		l.Errorf("set read deadline: %v", err)
		return fmt.Errorf("set read deadline: %w", err)
	}
	var wait bool
	c := chrony.Client{Sequence: 1, Connection: conn}

	log.Printf("connected to chronyd ok; starting loop")
	for {
		if wait {
			time.Sleep(time.Minute)
		} else {
			wait = true
		}
		ts := time.Now().UnixNano()

		treq := chrony.NewTrackingPacket()
		tres, err := c.Communicate(treq)
		if err != nil {
			return fmt.Errorf("get tracking info: communicate: %w", err)
		}
		tracking, ok := tres.(*chrony.ReplyTracking)
		if ok {
			l.Printf("tracking: %#v", tracking)
			line := fmt.Sprintf(`tracking,machine=%s ref_id="%x",stratum=%vu,leap_status=%vu,reftime=%vu,correction=%v,offset=%v,rms_offset=%v,freq_ppm=%v,residual_freq_ppm=%v,skew=%v,root_delay=%v,root_dispersion=%v,update_interval=%v %v`, source, tracking.RefID, tracking.Stratum, tracking.LeapStatus, tracking.RefTime.UnixNano(), tracking.CurrentCorrection, tracking.LastOffset, tracking.RMSOffset, tracking.FreqPPM, tracking.ResidFreqPPM, tracking.SkewPPM, tracking.RootDelay, tracking.LastUpdateInterval, tracking.RootDispersion, ts)
			if err := sendToInflux(line); err != nil {
				l.Errorf("get tracking info: problem sending to influx: %v", err)
			}
		} else {
			l.Errorf("tracking reply was of unexpected type: %#v", tres)
		}

		ssreq := chrony.NewSourcesPacket()
		ssres, err := c.Communicate(ssreq)
		if err != nil {
			return fmt.Errorf("get sources: %w", err)
		}
		var sources int
		if ss, ok := ssres.(*chrony.ReplySources); ok {
			l.Printf("sources: %#v", ss)
			sources = ss.NSources
		} else {
			l.Errorf("sources reply was of unexpected type: %#v", ssres)
			sources = 0
		}
		for i := 0; i < sources; i++ {
			sreq := chrony.NewSourceDataPacket(int32(i))
			sres, err := c.Communicate(sreq)
			if err != nil {
				return fmt.Errorf("get sources: %w", err)
			}
			s, ok := sres.(*chrony.ReplySourceData)
			if !ok {
				l.Errorf("source %v: source reply was of unexpected type: %#v", i, sres)
				continue
			}
			l.Printf("source %d (%v): %#v", i, refID(s.IPAddr), s)
			line := fmt.Sprintf("source,machine=%s,source=%s poll=%vi,stratum=%vu,state=%vu,mode=%vu,flags=%vu,reachability=%vu,since_sample=%vu,orig_latest_meas=%v,latest_meas=%v,latest_meas_err=%v %v", source, refID(s.IPAddr), s.Poll, s.Stratum, s.State, s.Mode, s.Flags, s.Reachability, s.SinceSample, s.OrigLatestMeas, s.LatestMeas, s.LatestMeasErr, ts)
			if err := sendToInflux(line); err != nil {
				l.Errorf("source %v: problem sending to influx: %v", i, err)
			}
		}
	}
}

func refID(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		last := len(v4)
		for i, b := range v4 {
			if b == 0 && i > 0 {
				last = i
				break
			}
			if b < '0' || b > 'z' {
				last = 0
				break
			}
		}
		if last > 0 {
			return string(v4[0:last])
		}
	}
	return ip.String()
}
