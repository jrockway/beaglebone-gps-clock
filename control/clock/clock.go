// Package clock creates an image to display on the face of the clock.
package clock

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"time"

	"github.com/jrockway/beaglebone-gps-clock/control/fixed58"
	"github.com/jrockway/beaglebone-gps-clock/control/screen"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func toNanos(ds []time.Duration) []float64 {
	var result []float64
	for _, d := range ds {
		result = append(result, float64(d.Nanoseconds()))
	}
	return result
}

var (
	missedTicksCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "missed_ticks",
		Help: "count of ticks that were generated but never received by anything",
	})

	tickDelayMetric = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "tick_delay",
		Help:    "amount of time between seconds tick and when it is sent to the channel, in nanoseconds",
		Buckets: prometheus.ExponentialBuckets(1000, 10, 20),
	})
)

// Tick sends the current time to the provided channel at the exact instant that the seconds change.
// An absent listener will not receive an outdated time; the tick will be skipped and the
// missedTicksCounter incremented.  Cancelling the context causes this to return immediately.
func Tick(ctx context.Context, ch chan time.Time) error {
	for {
		nextSecond := time.Now().Add(time.Second).Truncate(time.Second)

		// Wait until the next second starts.
		select {
		case <-time.After(time.Until(nextSecond)):
		case <-ctx.Done():
			return fmt.Errorf("waiting for next second: %w", ctx.Err())
		}

		// Send the time to the channel.
		select {
		case <-time.After(500 * time.Millisecond):
			missedTicksCounter.Inc()
		case <-ctx.Done():
			return fmt.Errorf("waiting to send tick: %w", ctx.Err())
		case ch <- nextSecond:
			tickDelayMetric.Observe(float64(time.Since(nextSecond).Nanoseconds()))
		}
	}
}

func renderTime(img *image.NRGBA64, t time.Time, c color.Color) {
	drawer := &font.Drawer{
		Dst: img,
		Src: image.NewUniform(c),
		Face: &basicfont.Face{
			Advance: 5,
			Width:   5,
			Height:  8,
			Ascent:  8,
			Descent: 0,
			Mask:    fixed58.Mask5x8,
			Ranges: []basicfont.Range{
				{'\u0020', '\u007f', 0},
				{'\ufffd', '\ufffe', 95},
			},
		},
		Dot: fixed.P(0, 8),
	}
	drawer.DrawString(t.Format("15:04:05"))
}

// Clock represents a clock face with parameters that can be changed at runtime.
type Clock struct {
	display      *screen.Screen
	BrightnessCh chan uint16
}

func New(d *screen.Screen) *Clock {
	return &Clock{display: d, BrightnessCh: make(chan uint16)}
}

// Run runs the clock until the context is cancelled.
func (c *Clock) Run(ctx context.Context) error {
	brightness := uint16(0xffff)
	t := time.Now()

	tickErrCh := make(chan error)
	tickCh := make(chan time.Time)
	go func() {
		err := Tick(ctx, tickCh)
		select {
		case tickErrCh <- err:
		case <-ctx.Done():
		}
		close(tickErrCh)
		close(tickCh)
	}()
	for {
		select {
		case t = <-tickCh:
		case err := <-tickErrCh:
			return fmt.Errorf("ticker: %w", err)
		case brightness = <-c.BrightnessCh:
		}
		img := c.display.EmptyCanvas()
		renderTime(img, t, color.NRGBA64{R: 0xffff, G: 0xffff, B: 0xffff, A: brightness})
		c.display.Display(img)
	}
}
