package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/facebookincubator/ntp/protocol/chrony"
	"github.com/stratoberry/go-gpsd"
)

var (
	statusMu sync.RWMutex
	status   Status

	//go:embed index.html.tmpl
	indexHTML string
	funcMap   = template.FuncMap{
		"hex":        formatHex,
		"unixtime":   formatUnixTime,
		"refid":      formatRefID,
		"duration":   formatDuration,
		"float3":     formatFloat3,
		"leap":       formatLeap,
		"correction": formatCorrection,
		"freq":       formatFreq,
		"source":     formatSource,
		"image":      formatImage,
	}
	index = template.Must(template.New("index").Funcs(funcMap).Parse(indexHTML))
)

type Status struct {
	ClockFace  *image.RGBA
	Now        time.Time
	Tracking   chrony.ReplyTracking
	Sources    []chrony.ReplySourceData
	Satellites []gpsd.Satellite
}

func UpdateStatus(newStatus Status) {
	statusMu.Lock()
	defer statusMu.Unlock()
	if newStatus.ClockFace != nil {
		status.ClockFace = newStatus.ClockFace
	}
	if newStatus.Tracking.Command != 0 {
		status.Tracking = newStatus.Tracking
	}
	if len(newStatus.Sources) != 0 {
		status.Sources = newStatus.Sources
	}
	if !newStatus.Now.IsZero() {
		status.Now = newStatus.Now
	}
	if len(newStatus.Satellites) > 0 {
		status.Satellites = newStatus.Satellites
	}
}

func ServeStatus(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	statusMu.RLock()
	defer statusMu.RUnlock()
	if err := index.Execute(w, status); err != nil {
		log.Printf("execute template: %v", err)
	}
}

func formatHex(x interface{}) string { return fmt.Sprintf("%x", x) }

func formatUnixTime(t time.Time) string { return t.In(time.UTC).Format(time.UnixDate) }

func formatRefID(x uint32) string {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, x)
	return refID(ip)
}

func formatDuration(x float64) string {
	d := time.Duration(x * 1e9)
	return d.String()
}

func formatLeap(x uint16) string {
	// From chrony/client.c and chrony/ntp.h
	switch x {
	case 0:
		return "Normal"
	case 1:
		return "Insert second"
	case 2:
		return "Delete second"
	case 3:
		return "Unsynchronized"
	default:
		return fmt.Sprintf("Invalid (%v)", x)
	}
}

func formatCorrection(x float64) string {
	var fast string
	if x < 0 {
		x = -x
		fast = "fast"
	} else {
		fast = "slow"
	}
	return fmt.Sprintf("%s %s of NTP time", time.Duration(x*1e9).String(), fast)
}

func formatFreq(x float64) string {
	var fast string
	if x < 0 {
		x = -x
		fast = "slow"
	} else {
		fast = "fast"
	}
	return fmt.Sprintf("%.3f ppm %s", x, fast)
}

func formatSource(x chrony.ReplySourceData) string {
	mode, state := " ", " "
	switch x.Mode {
	case chrony.SourceModeClient:
		mode = "^"
	case chrony.SourceModePeer:
		mode = "="
	case chrony.SourceModeRef:
		mode = "#"
	}

	// I think the upstream library made a mistake here:
	// cadnm.h                   packet.go
	// RPY_SD_ST_SELECTED      0 SourceStateSync
	// RPY_SD_ST_NONSELECTABLE 1 SourceStateUnreach
	// RPY_SD_ST_FALSETICKER   2 SourceStateFalseTicket
	// RPY_SD_ST_JITTERY       3 SourceStateJittery
	// RPY_SD_ST_UNSELECTED    4 SourceStateCandidate
	// RPY_SD_ST_SELECTABLE    5 SourceStateOutlier
	switch x.State {
	case chrony.SourceStateSync:
		state = "*"
	case chrony.SourceStateUnreach:
		state = "?"
	case chrony.SourceStateFalseTicket:
		state = "x"
	case chrony.SourceStateJittery:
		state = "~"
	case chrony.SourceStateCandidate:
		state = "+"
	case chrony.SourceStateOutlier:
		state = "-"
	}
	name := refID(x.IPAddr)
	if len(name) > 27 {
		name = name[:27]
	}
	return fmt.Sprintf("%s%s %-27s  %2d   %2d   %08b  %13s  %13s[%13s] +/- %13s\n", mode, state, name, x.Stratum, x.Poll, x.Reachability, time.Duration(1e9*x.SinceSample), time.Duration(1e9*x.LatestMeas), time.Duration(1e9*x.OrigLatestMeas), time.Duration(1e9*x.LatestMeasErr))
}

func formatImage(src *image.RGBA) template.URL {
	enlarge, space := 16, 2
	if src == nil {
		src = image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	img := image.NewRGBA(image.Rect(0, 0, enlarge*src.Bounds().Dx(), enlarge*src.Bounds().Dy()))
	for x := 0; x < src.Bounds().Dx(); x++ {
		for y := 0; y < src.Bounds().Dy(); y++ {
			val := src.At(x, y)
			for i := space; i < enlarge-space; i++ {
				for j := space; j < enlarge-space; j++ {
					img.Set(x*enlarge+i, y*enlarge+j, val)
				}
			}
		}
	}

	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		log.Printf("problem encoding image: %v", err)
		return template.URL("data:text/plain,error")
	}
	return template.URL("data:image/png;base64," + base64.RawStdEncoding.EncodeToString(buf.Bytes()))
}

func formatFloat3(x float64) string { return fmt.Sprintf("%.3f", x) }
