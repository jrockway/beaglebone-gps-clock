package main

import (
	"log"
	"time"
)

func main() {
	if err := monitorSensors(); err != nil {
		log.Fatalf("init sensors: %v", err)
	}

	if err := drawClock(); err != nil {
		log.Fatalf("init clock: %v", err)
	}

	for {
		if err := monitorChrony(); err != nil {
			log.Printf("monitor chrony exited unexpectedly: %v", err)
			time.Sleep(10 * time.Second)
		}
	}
}
