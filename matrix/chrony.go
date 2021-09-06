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

type SourceInfo struct {
	Index int
	Data  chrony.SourceData
	Stats chrony.SourceStats
}

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

	c := chrony.Client{Sequence: 1, Connection: conn}
	log.Printf("connected to chronyd ok; starting loop")
	var wait bool
	for {
		if wait {
			time.Sleep(30 * time.Second)
		} else {
			wait = true
		}
		deadline := time.Now().Add(time.Minute)
		if err := conn.SetReadDeadline(deadline); err != nil {
			l.Errorf("set read deadline: %v", err)
			return fmt.Errorf("set read deadline: %w", err)
		}
		l.Printf("extended read deadline to %v", deadline.Format("15:04:05.000000"))
		ts := time.Now().UnixNano()

		treq := chrony.NewTrackingPacket()
		tres, err := c.Communicate(treq)
		if err != nil {
			return fmt.Errorf("get tracking info: communicate: %w", err)
		}
		tracking, ok := tres.(*chrony.ReplyTracking)
		if ok {
			UpdateStatus(Status{Tracking: *tracking, Now: time.Now()})
			l.Printf("tracking: %#v", tracking)
			line := fmt.Sprintf(`tracking,machine=%s ref_id="%x",stratum=%vu,leap_status=%vu,reftime=%vu,correction=%v,offset=%v,rms_offset=%v,freq_ppm=%v,residual_freq_ppm=%v,skew=%v,root_delay=%v,root_dispersion=%v,update_interval=%v %v`, source, tracking.RefID, tracking.Stratum, tracking.LeapStatus, tracking.RefTime.UnixNano(), tracking.CurrentCorrection, tracking.LastOffset, tracking.RMSOffset, tracking.FreqPPM, tracking.ResidFreqPPM, tracking.SkewPPM, tracking.RootDelay, tracking.LastUpdateInterval, tracking.RootDispersion, ts)
			if err := sendToInflux(line); err != nil {
				l.Errorf("get tracking info: problem sending to influx: %v", err)
			}
		} else {
			l.Errorf("tracking reply was of unexpected type: %#v", tres)
		}

		ssreq := chrony.NewSourcesPacket()
		sres, err := c.Communicate(ssreq)
		if err != nil {
			return fmt.Errorf("get sources: %w", err)
		}
		var sources int
		if s, ok := sres.(*chrony.ReplySources); ok {
			l.Printf("sources: %#v", s)
			sources = s.NSources
		} else {
			l.Errorf("sources reply was of unexpected type: %#v", sres)
			sources = 0
		}

		// Chrony seems to keep sources sorted consistently in its own output, so I assume
		// its internal representation allows it to return matching source data and
		// sourcestats for lookups of the same index.  Not so sure that I wouldn't not write
		// this comment though I guess.
		var info []SourceInfo
		for i := 0; i < sources; i++ {
			// Get source data.
			sdreq := chrony.NewSourceDataPacket(int32(i))
			sdres, err := c.Communicate(sdreq)
			if err != nil {
				return fmt.Errorf("get sources: %w", err)
			}
			sd, ok := sdres.(*chrony.ReplySourceData)
			if !ok {
				l.Errorf("source %v: source data reply was of unexpected type: %#v", i, sdres)
				continue
			}

			// Get sourcestats.
			ssreq := chrony.NewSourceStatsPacket(int32(i))
			ssres, err := c.Communicate(ssreq)
			if err != nil {
				return fmt.Errorf("get sourcestats: %w", err)
			}
			ss, ok := ssres.(*chrony.ReplySourceStats)
			if !ok {
				l.Errorf("source %v: sourcestats reply was of unexpected type: %#v", i, ssres)
				continue
			}

			// Send both as one row.
			l.Printf("source %v (%v):\n    data: %#v\n    stats: %#v", i, refID(sd.IPAddr), sd, ss)
			line := fmt.Sprintf("source,machine=%s,source=%s poll=%vi,stratum=%vu,state=%vu,mode=%vu,flags=%vu,reachability=%vu,since_sample=%vu,orig_latest_meas=%v,latest_meas=%v,latest_meas_err=%v,samples=%vu,runs=%vu,span=%vu,resid_freq_ppm=%v,skew_ppm=%v,estimated_offset=%v,estimated_offset_err=%v,standard_deviation=%v %v", source, refID(sd.IPAddr), sd.Poll, sd.Stratum, sd.State, sd.Mode, sd.Flags, sd.Reachability, sd.SinceSample, sd.OrigLatestMeas, sd.LatestMeas, sd.LatestMeasErr, ss.NSamples, ss.NRuns, ss.SpanSeconds, ss.ResidFreqPPM, ss.SkewPPM, ss.EstimatedOffset, ss.EstimatedOffsetErr, ss.StandardDeviation, ts)
			if err := sendToInflux(line); err != nil {
				l.Errorf("source %v: problem sending to influx: %v", i, err)
			}
			info = append(info, SourceInfo{
				Index: i,
				Data:  sd.SourceData,
				Stats: ss.SourceStats,
			})
		}
		UpdateStatus(Status{Sources: info})
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

func intRefID(ip uint32) string {
	return refID(net.IPv4(byte((ip>>24)&0xff), byte((ip>>16)&0xff), byte((ip>>8)&0xff), byte(ip&0xff)))
}
