package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jrockway/beaglebone-gps-clock/control/clock"
	"github.com/jrockway/beaglebone-gps-clock/control/screen"
	"github.com/jrockway/periphflag"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"periph.io/x/extra/hostextra"
	"periph.io/x/periph/conn/spi/spireg"
)

var (
	bind = flag.String("bind", ":8080", "address to bind for debug/metrics server")
	spi  string
)

func main() {
	if _, err := hostextra.Init(); err != nil {
		log.Fatalf("init periph.io: %v", err)
	}
	periphflag.SPIDevVar(&spi, "spi", "", "spi bus that the display is on")
	flag.Parse()

	spiPort, err := spireg.Open(spi)
	if err != nil {
		log.Fatalf("open spi port %q: %v", spi, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	leds, err := screen.NewScreen(spiPort)
	if err != nil {
		log.Fatalf("init screen: %v", err)
	}
	leds.Blank()

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/display.png", http.StatusFound)
	})
	http.Handle("/display.png", leds)
	http.Handle("/metrics", promhttp.Handler())

	httpDoneCh := make(chan error)
	httpServer := http.Server{Addr: *bind}
	go func() {
		log.Printf("http server listening on %s", httpServer.Addr)
		err := httpServer.ListenAndServe()
		select {
		case httpDoneCh <- err:
		case <-ctx.Done():
		}
		close(httpDoneCh)
	}()

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	cl := clock.New(leds)
	loopDoneCh := make(chan error)
	go func() {
		err := cl.Run(ctx)
		select {
		case loopDoneCh <- err:
		case <-ctx.Done():
		}
		close(loopDoneCh)
	}()

	cl.BrightnessCh <- 0x1000

	httpAlive := true
	select {
	case err := <-httpDoneCh:
		log.Printf("http server died: %v", err)
		httpAlive = false
	case err := <-loopDoneCh:
		log.Printf("clock loop died: %v", err)
	case <-sigCh:
		log.Printf("interrupt")
	}
	signal.Stop(sigCh)
	cancel()
	leds.Blank()
	if httpAlive {
		tctx, c := context.WithTimeout(context.Background(), time.Second)
		httpServer.Shutdown(tctx)
		c()
	}
	os.Exit(1)
}
