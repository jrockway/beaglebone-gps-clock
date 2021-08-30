package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"golang.org/x/net/trace"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/host/v3"
)

func main() {
	trace.AuthRequest = func(req *http.Request) (any bool, sensitive bool) {
		return true, true
	}

	if _, err := host.Init(); err != nil {
		log.Fatalf("init host: %v", err)
	}

	i2cBus, err := i2creg.Open("2")
	if err != nil {
		log.Printf("open i2c: %v", err)
		log.Printf("not monitoring sensors")
	}

	if i2cBus != nil {
		if err := monitorSensors(i2cBus); err != nil {
			log.Fatalf("init sensors: %v", err)
		}
	}

	go drawClock()
	go watchGpsd()
	go watchChrony()

	log.Println("listening on :8080")
	http.HandleFunc("/", ServeStatus)
	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		log.Println(err)
	}
}
