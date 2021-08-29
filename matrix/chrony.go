package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/facebookincubator/ntp/protocol/chrony"
)

const source = "beaglebone"

func monitorChrony() error {
	conn, err := net.DialTimeout("udp", "localhost:323", time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}
	var wait bool
	c := chrony.Client{Sequence: 1, Connection: conn}

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
			line := fmt.Sprintf(`tracking,machine=%s ref_id="%x",stratum=%vu,leap_status=%vu,reftime=%vu,correction=%v,offset=%v,rms_offset=%v,freq_ppm=%v,residual_freq_ppm=%v,skew=%v,root_delay=%v,root_dispersion=%v,update_interval=%v %v`, source, tracking.RefID, tracking.Stratum, tracking.LeapStatus, tracking.RefTime.UnixNano(), tracking.CurrentCorrection, tracking.LastOffset, tracking.RMSOffset, tracking.FreqPPM, tracking.ResidFreqPPM, tracking.SkewPPM, tracking.RootDelay, tracking.LastUpdateInterval, tracking.RootDispersion, ts)
			if err := sendToInflux(line); err != nil {
				log.Printf("get tracking info: problem sending to influx: %v", err)
			}
		} else {
			log.Printf("tracking reply was of unexpected type: %#v", tres)
		}

		ssreq := chrony.NewSourcesPacket()
		ssres, err := c.Communicate(ssreq)
		if err != nil {
			return fmt.Errorf("get sources: %w", err)
		}
		var sources int
		if ss, ok := ssres.(*chrony.ReplySources); ok {
			sources = ss.NSources
		} else {
			log.Printf("sources reply was of unexpected type: %#v", ssres)
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
				log.Printf("source %v: source reply was of unexpected type: %#v", i, sres)
				continue
			}
			line := fmt.Sprintf("source,machine=%s,source=%s poll=%vi,stratum=%vu,state=%vu,mode=%vu,flags=%vu,reachability=%vu,since_sample=%vu,orig_latest_meas=%v,latest_meas=%v,latest_meas_err=%v %v", source, refID(s.IPAddr), s.Poll, s.Stratum, s.State, s.Mode, s.Flags, s.Reachability, s.SinceSample, s.OrigLatestMeas, s.LatestMeas, s.LatestMeasErr, ts)
			if err := sendToInflux(line); err != nil {
				log.Printf("source %v: problem sending to influx: %v", i, err)
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
