package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fulr/spidev"
)

func main() {
	here, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatal(err)
	}

	spi, err := spidev.NewSPIDevice("/dev/spidev0.0")
	if err != nil {
		log.Fatal(err)
	}

	spi.Xfer([]byte{0x0B, 0x07}) // scan limit
	spi.Xfer([]byte{0x09, 0xFF}) // decode mode
	spi.Xfer([]byte{0x0F, 0x00}) // display test off
	spi.Xfer([]byte{0x0C, 0x01}) // shutdown off
	spi.Xfer([]byte{0x0A, 0x01}) // brightness

	// blank digits
	spi.Xfer([]byte{0x06, 0x0F})
	spi.Xfer([]byte{0x03, 0x0F})

	log.Printf("clock initialized")
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

clock:
	for {
		now := time.Now().In(here)
		h, m, s := now.Clock()

		spi.Xfer([]byte{0x08, byte(h / 10)})
		spi.Xfer([]byte{0x07, byte((h % 10) | 0x80)})

		spi.Xfer([]byte{0x05, byte(m / 10)})
		spi.Xfer([]byte{0x04, byte((m % 10) | 0x80)})

		spi.Xfer([]byte{0x02, byte(s / 10)})
		spi.Xfer([]byte{0x01, byte(s % 10)})

		// Wake up again right as the next second starts.  This means that the display will
		// show the wrong second for about 500 microseconds on average (measured)... but
		// it's okay.
		next := time.Now().Add(time.Second).Truncate(time.Second).Sub(time.Now())
		select {
		case <-exit:
			break clock
		case <-time.After(next):
		}
	}
	log.Printf("exiting")

	// Blank all digits when exiting on a signal, just so someone looking at the clock can tell
	// whether the OS crashed or we just exited the program for some reason.
	spi.Xfer([]byte{0x08, 0x0F})
	spi.Xfer([]byte{0x07, 0x0F})
	spi.Xfer([]byte{0x06, 0x0F})
	spi.Xfer([]byte{0x05, 0x0F})
	spi.Xfer([]byte{0x04, 0x0F})
	spi.Xfer([]byte{0x03, 0x0F})
	spi.Xfer([]byte{0x02, 0x0F})
	spi.Xfer([]byte{0x01, 0x8F}) // period so that you can see the clock still has power.
}
