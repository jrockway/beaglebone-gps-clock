// Package screen draws images to my display, and retains them for debugging the rest of the program
// without the display attached.
package screen

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/http"
	"sync"

	"periph.io/x/periph/conn/spi"
	"periph.io/x/periph/devices/apa102"
)

const (
	rows                = 8
	cols                = 8
	panels              = 6
	previewScale        = 20 // Size of one pixel in the rendered image.
	previewPixelBorder  = 10 // Border around right and bottom of pixel, to simulate pixel spacing.
	previewPanelSpacing = 20 // Border between panels.

	idlePower  = 0.4174 * 5 // W
	powerLimit = 10         // W
)

// Screen represents the particular display I built for this project.  It consists of 6 8x8 grids of
// APA102 LEDs.  Each grid's 0th LED is in the top-left corner, and is column-major.  Odd-numbered
// grids are upside down.  The result is a pixel ordering like this:
//
// 0 8 ... 56 | 127 .. .. | 128 ...
// 1 . ... .. | 126 .. .. | ...
// 2 . ... .. | ... .. .. |
// 3 . ... .. | ... .. .. |
// 4 . ... .. | ... .. .. |
// 5 . ... .. | ... .. .. |
// 6 . ... .. | ... .. 65 |
// 7 . ... 63 | ... .. 64 |
//
// The panels come from two batches with wildly-different color correction curves.  This library
// applies the corrections to the panels.
//
// I used very small-guage wire and cannot actually provide the 5V * (8*8*6*60mA) = 115W that the
// display would require at full brightness with all pixels on.  Also everything would catch on
// fire.  So we "current limit" the display.
//
type Screen struct {
	leds *apa102.Dev

	imageMu sync.Mutex
	image   *image.NRGBA64 // must hold imageMu to read or write.
}

// NewScreen returns an initialized Screen object.
func NewScreen(p spi.Port) (*Screen, error) {
	s := &Screen{
		image: image.NewNRGBA64(image.Rect(0, 0, (panels-1)*previewPanelSpacing+panels*cols*(previewScale+previewPixelBorder), rows*(previewScale+previewPixelBorder))),
	}
	if p == nil {
		return s, nil
	}
	opts := &apa102.Opts{
		NumPixels:        rows * cols * panels,
		Intensity:        255,
		Temperature:      apa102.NeutralTemp,
		DisableGlobalPWM: true,
	}
	leds, err := apa102.New(p, opts)
	if err != nil {
		return nil, fmt.Errorf("init apa102: %w", err)
	}
	s.leds = leds
	return s, nil
}

// EmptyCanvas returns an image that's the right size for the display.
func (s *Screen) EmptyCanvas() *image.NRGBA64 {
	img := image.NewNRGBA64(image.Rect(0, 0, panels*cols, rows))
	for x := 0; x < panels*cols; x++ {
		for y := 0; y < rows; y++ {
			img.SetNRGBA64(x, y, color.NRGBA64{R: 0, G: 0, B: 0, A: 0xffff})
		}
	}
	return img
}

// Blank blanks the screen.
func (s *Screen) Blank() error {
	if err := s.Display(image.Black); err != nil {
		return fmt.Errorf("blank display: %w", err)
	}
	return nil
}

// ServeHTTP serves the current image as a PNG.
func (s *Screen) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("content-type", "image/png")
	w.WriteHeader(http.StatusOK)
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	if err := png.Encode(w, s.image); err != nil {
		log.Printf("encoding image: %v", err)
	}
}

// updateCurrentImage updates the image data that will be returned via the web interface.
func (s *Screen) updateCurrentImage(img image.Image) {
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	for x := 0; x < panels*cols; x++ {
		for y := 0; y < rows; y++ {
			r, g, b, a := img.At(x, y).RGBA()
			c := color.RGBA64{R: uint16(r), G: uint16(g), B: uint16(b), A: uint16(a)}
			scale := previewPixelBorder + previewScale
			xOff := (x / rows) * previewPanelSpacing
			for destX := xOff + scale*x; destX < xOff+scale*(x+1); destX++ {
				for destY := scale * y; destY < scale*(y+1); destY++ {
					if destX < xOff+scale*(x+1)-previewPixelBorder && destY < scale*(y+1)-previewPixelBorder {
						s.image.Set(destX, destY, c)
					}
				}
			}
		}
	}
}

// indexOf maps an (x,y) coordinate to the strand index of my particular display.
func indexOf(x, y int) int {
	panel := x / cols
	if panel%2 == 0 {
		return x*cols + y
	} else {
		pix := (x*cols + y) % (rows * cols)
		return (panel+1)*rows*cols - 1 - pix
	}
}

// powerFor returns the number of watts that displaying color c on the pixel at (x,y) will use.
//
// For the convenience of calling code, we neglect to include the full-off current of 1.09mA per
// pixel.
func powerFor(x, y int, c color.Color) float64 {
	// The datasheet says we'll use a maximum of 60mA per pixel, so we assume that displaying
	// the brighest red + blue + green is what causes that to happen.
	r, g, b, _ := c.RGBA()
	return .02 * 5 * (float64(r)/0xffff + float64(g)/0xffff + float64(b)/0xffff)
}

func gamma(c uint32) uint8 {
	u := float64(c) / 0xffff
	return uint8(255 * math.Pow((u+0.055)/(1.055), 2.4))
}

// colorCorrect maps a color.Color to the device color.
func colorCorrect(x, y int, c color.Color) color.NRGBA {
	r, g, b, _ := c.RGBA()
	panel := x / cols
	if panel < 4 {
		return color.NRGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: 0xff,
		}
	} else {
		return color.NRGBA{
			R: gamma(r),
			G: gamma(g),
			B: gamma(b),
			A: 0xff,
		}
	}
}

// toMatrix takes a cols*panels x rows image and converts it to a slice of colors to send to the
// apa102 strip.
//
// We use this opportunity to globally reduce the brightness of the display to stay within a pre-set
// power budget.  We do the transformation as a linear operation on Rec709 colors, which is probably
// a bad algorithm.
//
// It also does per-panel color correction.  Input pixels are 64-bit Rec709 colors, output pixels
// are device-native 24-bit colors.
func toMatrix(img image.Image) []color.NRGBA {
	result := make([]color.NRGBA, rows*cols*panels)

	// Calculate how much power displaying this image will use.
	var power float64
	for x := 0; x < cols*panels; x++ {
		for y := 0; y < rows; y++ {
			power += powerFor(x, y, img.At(x, y))
		}
	}

	// Then scale every pixel down by a constant factor (currentLimit/power) to ensure that the
	// display stays within its power envelope.
	fmt.Printf("power before scale: %vW\n", power+idlePower)

	scale := float64(1)
	if power > powerLimit {
		scale = (powerLimit / power)
	}
	power = 0
	for x := 0; x < cols*panels; x++ {
		for y := 0; y < rows; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			power += powerFor(x, y, img.At(x, y))
			result[indexOf(x, y)] = colorCorrect(x, y, color.NRGBA64{
				R: uint16(scale * float64(r)),
				G: uint16(scale * float64(g)),
				B: uint16(scale * float64(b)),
				A: 0xffff,
			})
		}
	}
	fmt.Printf(" power after scale: %vW\n", power+idlePower)
	return result
}

// Display displays the provided image on the screen.
func (s *Screen) Display(img image.Image) error {
	s.updateCurrentImage(img)
	if s.leds == nil {
		return nil
	}
	if _, err := s.leds.Write(apa102.ToRGB(toMatrix(img))); err != nil {
		return fmt.Errorf("write to apa102 strand: %w", err)
	}
	return nil
}
