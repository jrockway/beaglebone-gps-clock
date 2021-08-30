module github.com/jrockway/beaglebone-gps-clock

go 1.13

require (
	github.com/facebookincubator/ntp v0.0.0-20210811125121-43a17d4267ad
	github.com/fulr/spidev v0.0.0-20150210165549-524e13e3fac2
	github.com/goiot/devices v0.0.0-20160708214026-09d1226fc8ea
	github.com/jrockway/periphflag v0.0.0-20191020104359-a1cd7211ce99
	github.com/prometheus/client_golang v1.2.1
	github.com/stratoberry/go-gpsd v0.0.0-20161204231141-54ddcfa61f47
	golang.org/x/exp v0.0.0-20191030013958-a1ab85dbe136
	golang.org/x/image v0.0.0-20210220032944-ac19c3e999fb
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777
	periph.io/x/conn/v3 v3.6.8
	periph.io/x/devices/v3 v3.6.11
	periph.io/x/extra v0.0.0-20190805002851-353eec1a00ff
	periph.io/x/host/v3 v3.7.0
	periph.io/x/periph v3.6.2+incompatible
)
