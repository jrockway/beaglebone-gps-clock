// Package trimble reads data from a Trimble Resolution-T GPS:
// http://trl.trimble.com/docushare/dsweb/Get/Document-221342/ResolutionT_UG_2B_54655-05-ENG.pdf
package trimble

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
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

// Packetizer is an io.Writer that collects TSIP bytes and emits full packets for further processing.
type Packetizer struct {
	// Channel C produces TSIP packets, in the format of <id> <packet data> (no TSIP protocol
	// padding or stuffed DLE bytes).
	C chan []byte

	buf     []byte
	fullDLE bool // true if the last byte in buf is actually two DLE bytes
}

// Write collects TSIP bytes to be packetized.
func (p *Packetizer) Write(in []byte) (int, error) {
	fmt.Printf("got %d bytes\n", len(in))
	for _, b := range in {
		if b == TSIP_DLE {
			// A DLE byte indicates that we are about to read <DLE> <id> for a new
			// packet, <DLE> <ETX> for the end of the packet, or <DLE> <DLE> for a
			// literal DLE octet in the data.

			if len(p.buf) > 0 && p.buf[len(p.buf)-1] == TSIP_DLE && !p.fullDLE {
			}
		}

		if l := len(p.buf); l > 0 && !p.fullDLE && p.buf[l-1] == TSIP_DLE {
			switch b {
			case TSIP_ETX:
			search:
				for s, c := range p.buf {
					// Find the first DLE byte and send from there (usually 0).
					// Honestly this is kind of flaky because there is no
					// guarantee that TSIP is even in the buffer.  For exmaple,
					// gpspipe prints JSON to stdout at the beginning and then
					// switches to TSIP.  There could easily be a <DLE> byte in
					// that JSON, in which case we confuse the next stage with
					// garbage data.  But it will resynchronize on the next
					// packet.
					if c == TSIP_DLE && s+1 < l-1 {
						p.C <- p.buf[s+1 : l-1]
						break search
					}
				}
				p.buf = make([]byte, 0)
				continue
			case TSIP_DLE:
				// The last byte was DLE, so don't add another byte, and tell the
				// next iteration of the loop not to treat the DLE as anything other
				// than data.
				p.fullDLE = true
				continue

			}
		}

		p.buf = append(p.buf, b)
		p.fullDLE = false
	}
	return len(in), nil
}

// Packet represents a parsed TSIP packet.
type Packet struct {
	AllInView          *AllInView
	SignalLevel        SignalLevels
	RawMeasurement     *RawMeasurement
	PrimaryTiming      *PrimaryTiming
	SupplementalTiming *SupplementalTiming

	UnknownPacketID byte
}

// AllInView reports information about the receiver's currently-tracked satellites.
type AllInView struct {
	Status                 int
	AutoFix                bool
	PDOP, HDOP, VDOP, TDOP float32
	Satellites             []int
}

// SignalLevel represents a satellite signal strength in units configured in the receiver's NVRAM,
// either AMU or dB/Hz.  Negative values represent satellites not locked.  Zero represents a
// satellite that has not been acquired.
type SignalLevel float32

func (s SignalLevel) Level() float32 {
	if s < 0 {
		return -float32(s)
	} else {
		return float32(s)
	}
}
func (s SignalLevel) Locked() bool   { return s > 0 }
func (s SignalLevel) Acquired() bool { return s != 0 }

// SignalLevels is a map from the satellite number (PRN?) to the signal strength.
type SignalLevels map[int]SignalLevel

// RawMeasurement represents raw GPS measurement data.
type RawMeasurement struct {
	PRN               int
	SampleLength      time.Duration
	SignalLevel       SignalLevel
	CodePhase         float32
	Doppler           float32
	TimeOfMeasurement time.Duration
}

// PrimaryTiming represents a TSIP primary timing data packet.
type PrimaryTiming struct {
	TimeOfWeek  uint32
	WeekNumber  uint16
	UTCOffset   int16
	TimingFlags uint8
	Seconds     uint8
	Minutes     uint8
	Hours       uint8
	DayOfMonth  uint8
	Month       uint8
	Year        uint16
}

// SecondaryTiming represents a TSIP supplemental timing packet.
type SupplementalTiming struct {
	ReceiverMode                                    int
	SelfSurveyProgress                              int
	MinorAlarms                                     int
	GPSDecodingStatus                               int
	LocalClockBias, LocalClockBiasRate, Temperature float32
	Latitude, Longitude, Altitude                   float64
	QuantizationError                               float32
}

func ParsePacket(b []byte) (result *Packet, err error) {
	if len(b) < 2 {
		return nil, fmt.Errorf("not enough data in TSIP packet; got %d bytes, want at least 2 bytes", len(b))
	}
	id := b[0]
	b = b[1:]
	result = new(Packet)

	defer func() {
		// Avoid crashing the program on bad data that might not have been fully-validated.
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	switch id {
	case TSIP_PACKET_ALL_IN_VIEW:
		if len(b) < 18 {
			return result, errors.New("incomplete 'All-in-view satellite selection' packet")
		}

		raw := struct {
			FixBits                uint8
			PDOP, HDOP, VDOP, TDOP float32
		}{}

		r := bytes.NewReader(b)
		if err := binary.Read(r, binary.BigEndian, &raw); err != nil {
			return result, err
		}

		v := new(AllInView)
		result.AllInView = v

		v.Status = int(raw.FixBits & 0x7)
		v.AutoFix = (raw.FixBits&0x8)>>3 == 1
		svCount := int((raw.FixBits & 0xf0) >> 4)
		v.PDOP, v.HDOP, v.VDOP, v.TDOP = raw.PDOP, raw.HDOP, raw.VDOP, raw.TDOP

		for i := 0; i < svCount; i++ {
			var prn int8
			if err := binary.Read(r, binary.BigEndian, &prn); err != nil {
				return result, err
			}
			v.Satellites = append(v.Satellites, int(prn))
		}

	case TSIP_PACKET_SIGNAL:
		n := int(b[0])
		if len(b) < 1+n*5 {
			return result, errors.New("incomplete 'Signal level' packet")
		}

		result.SignalLevel = make(SignalLevels)
		r := bytes.NewReader(b[1:])
		for i := 0; i < n; i++ {
			raw := struct {
				PRN    uint8
				Signal float32
			}{}
			if err := binary.Read(r, binary.BigEndian, &raw); err != nil {
				return result, err
			}
			result.SignalLevel[int(raw.PRN)] = SignalLevel(raw.Signal)
		}

	case TSIP_PACKET_RAW_MEASUREMENT:
		if len(b) < 25 {
			return result, errors.New("incomplete 'Raw data measurement data' packet")
		}
		r := bytes.NewReader(b)
		raw := struct {
			PRN                                     uint8
			Length, SignalLevel, CodePhase, Doppler float32
			Time                                    float64
		}{}

		if err := binary.Read(r, binary.BigEndian, &raw); err != nil {
			return result, err
		}

		result.RawMeasurement = new(RawMeasurement)
		result.RawMeasurement.PRN = int(raw.PRN)
		result.RawMeasurement.SampleLength = time.Millisecond * time.Duration(raw.Length)
		result.RawMeasurement.SignalLevel = SignalLevel(raw.SignalLevel)
		result.RawMeasurement.CodePhase = raw.CodePhase
		result.RawMeasurement.Doppler = raw.Doppler
		result.RawMeasurement.TimeOfMeasurement = time.Second * time.Duration(raw.Time)

	case TSIP_PACKET_TIMING_SUPERPACKET:
		switch b[0] {
		case TSIP_PACKET_TIMING_PRIMARY:
			if len(b) != 17 {
				return result, fmt.Errorf("invalid primary timing packet length %d", len(b))
			}
			result.PrimaryTiming = new(PrimaryTiming)
			r := bytes.NewReader(b[1:])
			if err := binary.Read(r, binary.BigEndian, result.PrimaryTiming); err != nil {
				return result, err
			}

		case TSIP_PACKET_TIMING_SUPPLEMENTAL:
			if len(b) != 68 {
				return result, fmt.Errorf("invalid supplemental timing packet length %d", len(b))
			}
			r := bytes.NewReader(b[1:])
			raw := struct {
				ReceiverMode                  uint8
				Reserved                      uint8
				SelfSurveyProgress            uint8
				Reserved2                     uint32
				Reserved3                     uint16
				MinorAlarms                   uint16
				DecodingStatus                uint8
				Reserved4                     uint8
				SpareStatus1                  uint8
				SpareStatus2                  uint8
				LocalClockBias                float32
				LocalClockBiasRate            float32
				Reserved5                     uint32
				Reserved6                     float32
				Temperature                   float32
				Latitude, Longitude, Altitude float64
				QuantizationError             float32
				Spare                         uint32
			}{}
			if err := binary.Read(r, binary.BigEndian, &raw); err != nil {
				return result, err
			}
			st := new(SupplementalTiming)
			result.SupplementalTiming = st
			st.ReceiverMode = int(raw.ReceiverMode)
			st.SelfSurveyProgress = int(raw.SelfSurveyProgress)
			st.MinorAlarms = int(raw.MinorAlarms)
			st.GPSDecodingStatus = int(raw.DecodingStatus)
			st.LocalClockBias = raw.LocalClockBias
			st.LocalClockBiasRate = raw.LocalClockBiasRate
			st.Temperature = raw.Temperature
			st.Latitude = raw.Latitude
			st.Longitude = raw.Longitude
			st.Altitude = raw.Altitude
			st.QuantizationError = raw.QuantizationError
		}

	default:
		result.UnknownPacketID = id
	}

	return result, nil
}
