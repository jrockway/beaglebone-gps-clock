package main

import (
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
