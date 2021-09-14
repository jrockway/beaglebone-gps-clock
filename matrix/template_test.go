package main

import (
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/facebookincubator/ntp/protocol/chrony"
	"github.com/jrockway/go-gpsd"
)

func TestTemplate(t *testing.T) {
	now := time.Now()
	UpdateStatus(Status{
		ClockFace: getClockImage(now),
		Now:       now,
		Tracking: chrony.ReplyTracking{
			Tracking: chrony.Tracking{
				Stratum: 1,
				RefID:   0x41414100,
			},
		},
		Sources: []SourceInfo{
			{
				Index: 0,
				Data: chrony.SourceData{
					IPAddr:       net.IPv4(0x41, 0x41, 0x41, 0x00),
					Stratum:      0,
					Poll:         4,
					State:        chrony.SourceStateSync,
					Mode:         chrony.SourceModeRef,
					Reachability: 0o377,
					SinceSample:  10,
				},
				Stats: chrony.SourceStats{
					RefID:              0x41414100,
					IPAddr:             net.IPv4(0x41, 0x41, 0x41, 0x00),
					NSamples:           16,
					NRuns:              16,
					SpanSeconds:        60 * 16,
					StandardDeviation:  0.001,
					ResidFreqPPM:       -0.001,
					SkewPPM:            0.010,
					EstimatedOffset:    0.000000001,
					EstimatedOffsetErr: 0.001,
				},
			},
		},
	})
	AddSatellite("/dev/ttyS1", gpsd.Satellite{PRN: 1, Az: 0, El: 45, Ss: 20, Used: true})
	AddSatellite("/dev/ttyS1", gpsd.Satellite{PRN: 2, Az: 120, El: 50, Ss: 20, Used: true})
	AddSatellite("/dev/ttyS1", gpsd.Satellite{PRN: 3, Az: 240, El: 55, Ss: 20, Used: true})
	AddSatellite("/dev/ttyS2", gpsd.Satellite{PRN: 4, Az: 0, El: 45, Ss: 20, Used: true})
	AddSatellite("/dev/ttyS2", gpsd.Satellite{PRN: 5, Az: 90, El: 50, Ss: 20, Used: true})
	AddSatellite("/dev/ttyS2", gpsd.Satellite{PRN: 6, Az: 180, El: 55, Ss: 20, Used: false})
	AddSatellite("/dev/ttyS2", gpsd.Satellite{PRN: 7, Az: 270, El: 60, Ss: 20, Used: true})
	AddPosition("/dev/ttyS1", 40, -73, 30)
	AddPosition("/dev/ttyS1", 40.0000001, -73.0000001, 30)
	AddPosition("/dev/ttyS1", 40.0000010, -73.0000010, 30)
	AddPosition("/dev/ttyS1", 40.0000100, -73.0000100, 30)
	for i := 0; i < 65536; i++ {
		AddPosition("/dev/ttyS2", 40.0000100+rand.Float64(), -73.0000100+rand.Float64(), 30+rand.Float64())
	}

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	ServeStatus(rec, req)
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("render index.html: response code:\n  got: %v\n want: %v", got, want)
	}
	if os.Getenv("DUMP") != "" {
		if err := ioutil.WriteFile("../index.html", rec.Body.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("wrote index.html")
	}
}
