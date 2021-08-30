package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"golang.org/x/net/trace"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/devices/v3/bmxx80"
)

func monitorSensors(i2cBus i2c.Bus) error {
	tempOpts := bmxx80.Opts{Temperature: bmxx80.O16x, Pressure: bmxx80.O16x, Humidity: bmxx80.O16x}
	temp, err := bmxx80.NewI2C(i2cBus, 0x77, &tempOpts)
	if err != nil {
		return fmt.Errorf("init bme280: %w", err)
	}
	go func() {
		l := trace.NewEventLog("sensor", "environment")
		defer l.Finish()
		first := true
		log.Printf("starting bme280 loop")
		for {
			if first {
				first = false
			} else {
				time.Sleep(30 * time.Second)
			}
			var e physic.Env
			if err := temp.Sense(&e); err != nil {
				l.Errorf("error: read temperature: %v", err)
				continue
			}
			l.Printf("Temp: %v, Pressure: %v, Humidity: %v", e.Temperature, e.Pressure, e.Humidity)
			// Temperature is in nanokelvin.  Humidity is in 10ths of a micro% (!).  Pressure is in nanopascal.
			if err := sendToInflux(fmt.Sprintf("environment temperature=%vu,relative_humidity=%vu,pressure=%vu %v\n", int64(e.Temperature), int64(e.Humidity), int64(e.Pressure), time.Now().UnixNano())); err != nil {
				l.Errorf("error: write influx: %v", err)
				continue
			}

		}
	}()

	light := TSL2591{dev: i2c.Dev{Bus: i2cBus, Addr: 0x29}}
	go func() {
		l := trace.NewEventLog("sensor", "luminosity")
		defer l.Finish()
		id, err := light.GetDeviceID()
		if err != nil {
			l.Errorf("get device id: %v", err)
			return
		}
		if got, want := id, uint8(0x50); got != want {
			l.Errorf("device at 0x29 is not a TSL2591 (got: %x, want: %x)", got, want)
			return
		}
		if err := light.Enable(); err != nil {
			l.Errorf("enable tsl2591: %v", err)
			return
		}
		if err := light.SetGain(HighGain); err != nil {
			l.Errorf("adjust tsl2591 gain: %v", err)
			return
		}
		if err := light.SetIntegrationTime(IntegrationTime600ms); err != nil {
			l.Errorf("adjust tsl2591 integration time: %v", err)
			return
		}
		log.Printf("starting tsl2591 loop")
		for {
			time.Sleep(time.Second)
			both, ir, err := light.GetLuminosity()
			if err != nil {
				l.Errorf("read luminosity: %v", err)
				continue
			}
			l.Printf("luminosity: %v ir: %v", both, ir)
			// TODO: scale by gain before writing.
			if err := sendToInflux(fmt.Sprintf("luminosity combined=%vu,ir=%vu %v\n", both, ir, time.Now().UnixNano())); err != nil {
				l.Errorf("write influx: %v", err)
				continue
			}
		}
	}()
	return nil
}

type TSL2591 struct {
	dev  i2c.Dev
	gain Gain
	it   time.Duration
}

type Register uint8

const (
	RegisterDeviceID Register = 0x12
	RegisterEnable   Register = 0x00
	RegisterControl  Register = 0x01
	RegisterChan0Low Register = 0x14
	RegisterChan1Low Register = 0x16
)

type Gain uint8

const (
	LowGain    Gain = 0x00
	MediumGain Gain = 0x10
	HighGain   Gain = 0x20
	MaxGain    Gain = 0x30
)

const (
	CommandEnablePowerOff = 0x00 // nolint:deadcode
	CommandEnablePowerOn  = 0x01
	CommandEnableAEN      = 0x02
	CommandEnableAIEN     = 0x10
	CommandEnableNPIEN    = 0x80

	IntegrationTime100ms = 0x00
	IntegrationTime200ms = 0x01
	IntegrationTime300ms = 0x02
	IntegrationTime400ms = 0x03
	IntegrationTime500ms = 0x04
	IntegrationTime600ms = 0x05
)

func (t *TSL2591) ReadRegister(r Register, out interface{}) error {
	var buf [2]byte
	if err := t.dev.Tx([]byte{byte(0xA0 | r)}, buf[:]); err != nil {
		return fmt.Errorf("tx: %w", err)
	}
	//fmt.Printf("read 0x%x:\n%s\n", r, hex.Dump(buf[:]))
	reader := bytes.NewReader(buf[:])
	if err := binary.Read(reader, binary.LittleEndian, out); err != nil {
		return fmt.Errorf("binary.Read: %w", err)
	}
	return nil
}

func (t *TSL2591) WriteRegister(r Register, data ...byte) error {
	//fmt.Printf("write 0x%x:\n%s\n", r, hex.Dump(data))
	w := make([]byte, 1, len(data)+1)
	w[0] = byte(0xA0 | r)
	w = append(w, data...)
	if err := t.dev.Tx(w, nil); err != nil {
		return fmt.Errorf("tx: %w", err)
	}
	return nil
}

func (t *TSL2591) GetDeviceID() (uint8, error) {
	var result uint8
	if err := t.ReadRegister(RegisterDeviceID, &result); err != nil {
		return 0, fmt.Errorf("read register: %w", err)
	}
	return result, nil
}

func (t *TSL2591) Enable() error {
	if err := t.WriteRegister(RegisterEnable, CommandEnablePowerOn|CommandEnableAEN|CommandEnableAIEN|CommandEnableNPIEN); err != nil {
		return fmt.Errorf("write enable register: %w", err)
	}
	return nil
}

func (t *TSL2591) SetGain(gain Gain) error {
	var control uint8
	if err := t.ReadRegister(RegisterControl, &control); err != nil {
		return fmt.Errorf("read control register: %w", err)
	}
	control &= 0b11001111
	control |= uint8(gain)
	if err := t.WriteRegister(RegisterControl, control); err != nil {
		return fmt.Errorf("write control register: %w", err)
	}
	t.gain = gain
	return nil
}

func (t *TSL2591) SetIntegrationTime(it uint8) error {
	var control uint8
	if err := t.ReadRegister(RegisterControl, &control); err != nil {
		return fmt.Errorf("read control register: %w", err)
	}
	control &= 0b11111000
	control |= it
	if err := t.WriteRegister(RegisterControl, control); err != nil {
		return fmt.Errorf("write control register: %w", err)
	}
	switch it {
	case IntegrationTime100ms:
		t.it = 100 * time.Millisecond
	case IntegrationTime200ms:
		t.it = 200 * time.Millisecond
	case IntegrationTime300ms:
		t.it = 300 * time.Millisecond
	case IntegrationTime400ms:
		t.it = 400 * time.Millisecond
	case IntegrationTime500ms:
		t.it = 500 * time.Millisecond
	case IntegrationTime600ms:
		t.it = 600 * time.Millisecond
	}
	return nil
}

func (t *TSL2591) GetLuminosity() (uint16, uint16, error) {
	var chan0, chan1 uint16
	if err := t.ReadRegister(RegisterChan0Low, &chan0); err != nil {
		return 0, 0, fmt.Errorf("read chan0: %w", err)
	}
	if err := t.ReadRegister(RegisterChan1Low, &chan1); err != nil {
		return 0, 0, fmt.Errorf("read chan1: %w", err)
	}
	return chan0, chan1, nil
}

func (t *TSL2591) Lux(total, ir uint16) float64 {
	var gain float64
	switch t.gain {
	case LowGain:
		gain = 1.0
	case MediumGain:
		gain = 25.0
	case HighGain:
		gain = 428.0
	case MaxGain:
		gain = 9876.0
	}
	// Nobody likes this calculation apparently:
	// https://github.com/adafruit/Adafruit_TSL2591_Library/issues/14
	cpl := (gain * float64(t.it) / float64(time.Millisecond)) / 408.0
	return float64(total-ir) * (1.0 - float64(ir)/float64(total)) / cpl
}
