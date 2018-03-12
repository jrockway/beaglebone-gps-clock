package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"time"

	"github.com/pkg/term"
)

const (
	TSIP_DLE = 0x10
	TSIP_ETX = 0x03

	TSIP_PACKET_SIGNAL             = 0x47
	TSIP_PACKET_ALL_IN_VIEW        = 0x6d
	TSIP_PACKET_RAW_MEASUREMENT    = 0x5a
	TSIP_PACKET_TIMING_SUPERPACKET = 0x8f

	TSIP_PACKET_TIMING_PRIMARY      = 0xab
	TSIP_PACKET_TIMING_SUPPLEMENTAL = 0xac
)

// Read data from a Trimble Resolution-T GPS:
// http://trl.trimble.com/docushare/dsweb/Get/Document-221342/ResolutionT_UG_2B_54655-05-ENG.pdf

func hexdump(b []byte) {
	for row := 0; row*16 < len(b); row++ {
		for col := 0; col < 16 && row*16+col < len(b); col++ {
			fmt.Printf("%#x ", b[row*16+col])
		}
		fmt.Printf("\n")
	}
	fmt.Printf("\n")
}

func findPacket(b []byte) (start, end int, id byte, ok bool) {
	start = -1
	end = -1
	id = 0
	// Packets start with DLE id.  Packets end with DLE ETX.

	for i := 0; i < len(b); i++ {
		// DLE bytes inside the packet data are doubled.  If we see a lone DLE byte, then
		// it's a DLE <id> or DLE ETX sequence.
		//
		// This protocol is somewhat questionable as nothing stops us from starting our read
		// in the middle of one of these doubled DLE packets and assuming the next data byte
		// is the command ID of the next packet.  Sigh.
		if b[i] == TSIP_DLE {
			if i+1 >= len(b) {
				// We don't have enough data to understand this DLE byte.
				ok = false
				return
			}

			if b[i+1] == TSIP_DLE {
				// Skip over data.
				i++
				continue
			}

			if b[i+1] == TSIP_ETX {
				end = i + 1
				ok = true
				return
			} else {
				id = b[i+1]
				start = i
				i++
				continue
			}
		}
	}
	ok = false
	return
}

func parsePacket(id byte, b []byte) {
	switch id {
	case TSIP_PACKET_ALL_IN_VIEW:
		if len(b) < 18 {
			log.Printf("incomplete 'All-in-view satellite selection' packet")
			return
		}

		switch b[0] & 0x7 {
		case 1:
			log.Printf("1D clock fix")
		case 3:
			log.Printf("2D fix")
		case 4:
			log.Printf("3D fix")
		case 5:
			log.Printf("overdetermined clock fix")
		default:
			log.Printf("unknown fix")
		}

		log.Printf("auto fix? %v", ((b[0]&0x08)>>3) == 1)

		svs := (b[0] & 0xf0) >> 4
		log.Printf("%d satellites in view", svs)

		// bytes 1-4 PDOP, 5-8 HDOP, 9-12 VDOP, 13-16 TDOP
		bits := binary.BigEndian.Uint32(b[13:17])
		tdop := math.Float32frombits(bits)
		log.Printf("TDOP: %v", tdop)

	case TSIP_PACKET_SIGNAL:
		n := int(b[0])
		if len(b) < 1+n*5 {
			log.Printf("incomplete 'Signal level' packet")
			return
		}

		for i := 0; i < n; i++ {
			bytes := b[1+i*5 : 1+i*5+5]
			prn := bytes[0]
			bits := binary.BigEndian.Uint32(bytes[1:5])
			log.Printf("SV %d:\t%v", prn, math.Float32frombits(bits))
		}

	case TSIP_PACKET_RAW_MEASUREMENT:
		if len(b) < 25 {
			log.Printf("incomplete 'Raw data measurement data' packet")
			return
		}
		prn := b[0]
		sampleLen := math.Float32frombits(binary.BigEndian.Uint32(b[1:5]))
		level := math.Float32frombits(binary.BigEndian.Uint32(b[5:9]))
		phase := math.Float32frombits(binary.BigEndian.Uint32(b[9:13]))
		doppler := math.Float32frombits(binary.BigEndian.Uint32(b[13:17]))
		measurementTime := math.Float64frombits(binary.BigEndian.Uint64(b[17:25]))
		log.Printf("SV %d: sample length: %vms, signal level: %v, code phase: %v, doppler@L1: %v, time of measurement: %vs", prn, sampleLen, level, phase, doppler, measurementTime)

	case TSIP_PACKET_TIMING_SUPERPACKET:
		switch b[0] {
		case TSIP_PACKET_TIMING_PRIMARY:
			if len(b) != 17 {
				log.Printf("invalid primary timing packet length %d", len(b))
				return
			}
			tow := binary.BigEndian.Uint32(b[1:5])
			week := binary.BigEndian.Uint16(b[5:7])
			utcOffset := int16(binary.BigEndian.Uint16(b[7:9]))
			flag := b[9]
			s, m, h, d, mon := b[10], b[11], b[12], b[13], b[14]
			y := binary.BigEndian.Uint16(b[15:17])

			utc := 0x1 & flag
			ppsUtc := 0x2 & flag >> 1
			notSet := 0x4 & flag >> 2
			notHaveUtc := 0x8 & flag >> 3
			userSet := 0x10 & flag >> 4

			log.Printf("* time: tow: %d, week: %d, UTC offset: %d", tow, week, utcOffset)
			log.Printf("        %d/%d/%d %d:%d:%d", y, mon, d, h, m, s)
			log.Printf("   utc: %d, ppsUtc: %d, notSet: %d, notHaveUtc %d, userSet: %d", utc, ppsUtc, notSet, notHaveUtc, userSet)

		case TSIP_PACKET_TIMING_SUPPLEMENTAL:
			if len(b) != 68 {
				log.Printf("invalid supplemental timing packet length %d", len(b))
				return
			}
			log.Printf("receiver mode %d", b[1])
			log.Printf("self-survey progress: %d%%", b[3])
			log.Printf("gps decoding status: %d", b[12])

			err := math.Float32frombits(binary.BigEndian.Uint32(b[60:64]))
			log.Printf("pps quantization error: %v seconds", err)
		}

	default:
		log.Printf("unknown packet %#x", id)
	}
}

func main() {
	f, err := term.Open("/dev/gps0", term.Speed(9600), term.RawMode)
	if err != nil {
		log.Fatal(err)
	}

	buf := new(bytes.Buffer)
	for {
		avail, err := f.Available()
		if err != nil {
			log.Fatalf("available: %v", err)
		}
		if avail < 1 {
			time.Sleep(time.Millisecond)
			continue
		}

		r := &io.LimitedReader{R: f, N: int64(avail)}
		if _, err := buf.ReadFrom(r); err != nil {
			log.Fatal(err)
		}

	parse:
		for {
			data := buf.Bytes()
			start, end, id, ok := findPacket(data)
			if ok {
				parsePacket(id, data[start+2:end-1])
				buf = bytes.NewBuffer(data[end+1 : len(data)])
			}
			break parse
		}

	}
}
