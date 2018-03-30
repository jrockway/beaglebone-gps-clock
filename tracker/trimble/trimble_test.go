package trimble

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
)

func TestWrite(t *testing.T) {
	testData := []struct {
		input []byte
		want  []byte
	}{
		{
			input: nil,
			want:  nil,
		},
		{
			input: []byte{TSIP_DLE, 0x42, 0x00, TSIP_DLE, TSIP_ETX},
			want:  []byte{0x42, 0x00},
		},
		{
			input: []byte{TSIP_DLE, 0x42, 0x00, TSIP_DLE, TSIP_DLE, TSIP_DLE, TSIP_ETX},
			want:  []byte{0x42, 0x00, TSIP_DLE},
		},
		{
			input: []byte{TSIP_DLE, 0x42, TSIP_DLE, TSIP_DLE, 0x00, TSIP_DLE, TSIP_ETX},
			want:  []byte{0x42, TSIP_DLE, 0x00},
		},
		{
			input: []byte("garbage"),
			want:  nil,
		},
		{
			input: []byte{'f', 'o', 'o', TSIP_DLE, 0x42, 0x00, TSIP_DLE, TSIP_ETX},
			want:  []byte{0x42, 0x00},
		},
	}

	for i, test := range testData {
		p := Packetizer{C: make(chan []byte, 1)}
		p.Write(test.input) // we never return an error

		var got []byte
		select {
		case got = <-p.C:
		default:
			got = nil
		}

		if want := test.want; !reflect.DeepEqual(got, want) {
			t.Errorf("test %d:\n  got: %v\n want: %v", i, got, want)
		}
	}
}

func TestParsePacket(t *testing.T) {
	testData := []struct {
		input string
		want  *Packet
	}{
		{
			input: "QgA=",
			want: &Packet{
				UnknownPacketID: 0x42,
			},
		},
		{
			input: "bS0AAAAAAAAAAAAAAAAAAAAACBI=",
			want: &Packet{
				AllInView: &AllInView{
					Status:     5,
					AutoFix:    true,
					PDOP:       0,
					HDOP:       0,
					VDOP:       0,
					TDOP:       0,
					Satellites: []int{8, 18},
				},
			},
		},
		{
			input: "bQ0AAAAAAAAAAAAAAAAAAAAA",
			want: &Packet{
				AllInView: &AllInView{
					Status:     5,
					AutoFix:    true,
					PDOP:       0,
					HDOP:       0,
					VDOP:       0,
					TDOP:       0,
					Satellites: nil,
				},
			},
		},
		{
			input: "RwsIwa2ZmgtCNgAAB0INmZoTgAAAABHBrZmaFkIoAAASQiMzMxxCBAAAA4AAAAAeQiAAAAFCQMzN",
			want: &Packet{
				SignalLevel: SignalLevels{
					1:  48.2,
					3:  -0,
					7:  35.4,
					8:  -21.7,
					11: 45.5,
					17: -21.7,
					18: 40.8,
					19: -0,
					22: 42,
					28: 33,
					30: 40,
				},
			},
		},
		{
			input: "Wgs/gAAAQjYAAEZ65Z7B14TbQQ9qmCAAAAA=",
			want: &Packet{
				RawMeasurement: &RawMeasurement{
					PRN:               11,
					SampleLength:      1000000,
					SignalLevel:       45.5,
					CodePhase:         16057.404,
					Doppler:           -26.93987,
					TimeOfMeasurement: 257363000000000,
				},
			},
		},
		{
			input: "j6sAA+1TB8oAEgMFHRcbAwfi",
			want: &Packet{
				PrimaryTiming: &PrimaryTiming{
					TimeOfWeek:  257363,
					WeekNumber:  1994,
					UTCOffset:   18,
					TimingFlags: 3,
					Seconds:     5,
					Minutes:     29,
					Hours:       23,
					DayOfMonth:  27,
					Month:       3,
					Year:        2018,
				},
			},
		},
		{
			input: "j6wHAAAAAAAAAAAAAAAAAABHsfslRJ6WMQAAAAAAAAAAQgu6oAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEGJS0cAAAAA",
			want: &Packet{
				SupplementalTiming: &SupplementalTiming{
					ReceiverMode:       7,
					SelfSurveyProgress: 0,
					MinorAlarms:        0,
					GPSDecodingStatus:  0,
					LocalClockBias:     91126.29,
					LocalClockBiasRate: 1268.6935,
					Temperature:        34.93225,
					Latitude:           0,
					Longitude:          0,
					Altitude:           0,
					QuantizationError:  17.161757,
				},
			},
		},
		{
			input: "XBoAAAAAAAAASOqnIj3NKoFAVJgRAQAAAQ==",
			want: &Packet{
				TrackingStatus: &TrackingStatus{
					PRN:                26,
					Channel:            0,
					AcquisitionFlag:    0,
					EphemerisFlag:      0,
					SignalLevel:        0,
					LastMeasurement:    480569.06,
					Elevation:          0.100178726,
					Azimuth:            3.3217814,
					OldMeasurementFlag: 1,
					BadDataFlag:        0,
					DataCollectionFlag: 0,
				},
			},
		},
	}

	for i, test := range testData {
		input, err := base64.StdEncoding.DecodeString(test.input)
		if err != nil {
			t.Fatalf("test %d: base64: %v", i, err)
		}

		got, err := ParsePacket(input)
		if err != nil {
			t.Errorf("test %d: parse: %v", i, err)
		}

		if want := test.want; !reflect.DeepEqual(got, want) {
			gotJson, err := json.Marshal(got)
			if err != nil {
				t.Errorf("error formatting got (%v): %v", got, err)
			}
			wantJson, err := json.Marshal(want)
			if err != nil {
				t.Errorf("error formatting want (%v): %v", want, err)
			}

			t.Errorf("test %d:\n  got: %s\n want: %s", i, gotJson, wantJson)
		}
	}

}
