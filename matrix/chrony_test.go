package main

import (
	"net"
	"testing"
)

func TestRefID(t *testing.T) {
	testData := []struct {
		in   net.IP
		want string
	}{
		{nil, "<nil>"},
		{net.IPv4(0, 0, 0, 0), "0.0.0.0"},
		{net.IPv6interfacelocalallnodes, "ff01::1"},
		{net.IPv4(1, 2, 3, 4), "1.2.3.4"},
		{net.IPv4('A', 0, 0, 0), "A"},
		{net.IPv4(80, 80, 83, 0), "PPS"},
		{net.IPv4(71, 80, 83, 0), "GPS"},
		{net.IPv4(82, 84, 67, 0), "RTC"},
		{net.IPv4(80, 72, 67, 48), "PHC0"},
	}

	for _, test := range testData {
		t.Run(test.in.String(), func(t *testing.T) {
			got := refID(test.in)
			if want := test.want; got != want {
				t.Errorf("convert refid (%s):\n  got: %v\n want: %v", []byte(test.in), got, want)
			}
		})
	}
}
