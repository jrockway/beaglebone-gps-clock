package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"time"

	"github.com/goiot/devices/dotstar"
	"golang.org/x/exp/io/spi"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// n is the number of LEDs on the strip.
const (
	n = 8 * 8 * 6
)

var myFont = &basicfont.Face{
	Advance: 5,
	Width:   5,
	Height:  8,
	Ascent:  8,
	Descent: 0,
	Mask:    mask5x8,
	Ranges: []basicfont.Range{
		{'\u0020', '\u007f', 0},
		{'\ufffd', '\ufffe', 95},
	},
}

func drawClock() error {
	d, err := dotstar.Open(&spi.Devfs{Dev: "/dev/spidev0.0", Mode: spi.Mode3}, n)
	if err != nil {
		return fmt.Errorf("open dotstar: %w", err)
	}

	// Blank the display.
	for i := 0; i < 6*8*8; i++ {
		d.SetRGBA(i, dotstar.RGBA{0, 0, 0, 0})
	}
	d.Draw()

	go func() {
		for {
			// Render the current time.
			img := image.NewRGBA(image.Rect(0, 0, 48, 8))
			(&font.Drawer{
				Dst:  img,
				Src:  image.NewUniform(color.RGBA{R: 0x20, G: 0xa0, B: 0xff, A: 0xff}),
				Face: myFont,
				Dot:  fixed.Point26_6{fixed.Int26_6(0), fixed.Int26_6(540)},
			}).DrawString(time.Now().Format("15:04:05"))
			for _, matrix := range []int{0, 2, 4} {
				i := matrix * 64
				for x := matrix * 8; x < (matrix+1)*8; x++ {
					for y := 0; y < 8; y++ {
						r, g, b, _ := img.At(x, y).RGBA()
						scale := byte(4)
						if matrix > 3 {
							scale = 50
						}
						d.SetRGBA(i, dotstar.RGBA{byte(r) / scale, byte(g) / scale, byte(b) / scale, 5})
						i++
					}
				}
			}
			for _, matrix := range []int{1, 3, 5} {
				i := matrix * 64
				for x := (matrix+1)*8 - 1; x >= matrix*8; x-- {
					for y := 7; y >= 0; y-- {
						r, g, b, _ := img.At(x, y).RGBA()
						scale := byte(4)
						if matrix > 3 {
							scale = 50
						}
						d.SetRGBA(i, dotstar.RGBA{byte(r) / scale, byte(g) / scale, byte(b) / scale, 5})
						i++
					}
				}
			}
			d.Draw()
			log.Printf("sleeping for %s", time.Until(time.Now().Add(time.Second).Truncate(time.Second)).String())
			time.Sleep(time.Until(time.Now().Add(time.Second).Truncate(time.Second)))
		}
	}()
	return nil
}
