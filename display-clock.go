package main

import (
	"time"

	"github.com/fulr/spidev"
)

func main() {
	here, err := time.LoadLocation("America/New_York")
	if err != nil {
		return
	}

	spi, err := spidev.NewSPIDevice("/dev/spidev0.0")
	if err != nil {
		return
	}

	spi.Xfer([]byte{0x0B, 0x07}) // scan limit
	spi.Xfer([]byte{0x09, 0xFF}) // decode mode
	spi.Xfer([]byte{0x0F, 0x00}) // display test off
	spi.Xfer([]byte{0x0C, 0x01}) // shutdown off
	spi.Xfer([]byte{0x0A, 0x0F}) // super bright

	spi.Xfer([]byte{0x08, 0x0F})
	spi.Xfer([]byte{0x07, 0x0F})

	for {
		now := time.Now().In(here)
		h, m, s := now.Clock()

		spi.Xfer([]byte{0x06, byte(h / 10)})
		spi.Xfer([]byte{0x05, byte((h % 10) | 0x80)})
		spi.Xfer([]byte{0x04, byte(m / 10)})
		spi.Xfer([]byte{0x03, byte((m % 10) | 0x80)})
		spi.Xfer([]byte{0x02, byte(s / 10)})
		spi.Xfer([]byte{0x01, byte(s % 10)})

		time.Sleep(10 * time.Millisecond)
	}
}
