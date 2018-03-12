package main

import "testing"

func TestFindPacket(t *testing.T) {
	testData := []struct {
		input      []byte
		start, end int
		id         byte
		ok         bool
	}{
		{[]byte{TSIP_DLE, 0x42, TSIP_DLE, TSIP_ETX}, 0, 3, 0x42, true},
		{[]byte{TSIP_DLE, 0x42, TSIP_DLE, TSIP_DLE, TSIP_DLE, TSIP_ETX}, 0, 5, 0x42, true},
		{[]byte{0, 0, TSIP_DLE, 0x41, 0, TSIP_DLE, TSIP_ETX, TSIP_DLE, 0x42}, 2, 6, 0x41, true},
		{[]byte{0, 0, TSIP_DLE, TSIP_DLE, 0x41, 0, TSIP_DLE, TSIP_ETX, TSIP_DLE, 0x42}, -1, -1, 0, false},
		{[]byte{TSIP_DLE}, -1, -1, 0, false},
		{[]byte{TSIP_DLE, 0x42, TSIP_DLE}, 0, -1, 0x42, false},
	}

	for i, test := range testData {
		start, end, id, ok := findPacket(test.input)

		if got, want := start, test.start; got != want {
			t.Errorf("test %d: start:\n  got: %d\n want: %d", i, got, want)
		}

		if got, want := end, test.end; got != want {
			t.Errorf("test %d: end:\n  got: %d\n want: %d", i, got, want)
		}

		if got, want := id, test.id; got != want {
			t.Errorf("test %d: id:\n  got: %#x\n want: %#x", i, got, want)
		}

		if got, want := ok, test.ok; got != want {
			t.Errorf("test %d: ok:\n  got: %v\n want: %v", i, got, want)
		}
	}
}
