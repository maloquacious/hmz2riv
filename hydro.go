// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"fmt"
	"math"

	"github.com/maloquacious/dem2hm"
)

// Flow direction codes. Codes 1 through 8 name the neighbor a pixel drains
// into, clockwise from east.
const (
	dirNone uint8 = 0  // no-data pixel
	dirOut  uint8 = 9  // drains off the map into a no-data pixel or the raster edge
	dirFlat uint8 = 10 // not yet assigned; only used while computing
)

var (
	nbrDX = [9]int{0, 1, 1, 0, -1, -1, -1, 0, 1}
	nbrDY = [9]int{0, 0, 1, 1, 1, 0, -1, -1, -1}
)

// nbrDist is the distance to each neighbor, in pixels.
var nbrDist = [9]float64{0, 1, math.Sqrt2, 1, math.Sqrt2, 1, math.Sqrt2, 1, math.Sqrt2}

// Hydrology is the drainage of a heightmap: filled elevations, a flow
// direction for every valid pixel, and the number of pixels that drain
// through each pixel.
type Hydrology struct {
	Width, Height int
	// Elev holds the depression-filled elevations in meters.
	Elev []int16
	// Dir holds each pixel's flow direction code.
	Dir []uint8
	// Acc holds each pixel's drainage area in pixels, counting itself.
	Acc []uint32

	// FilledPixels is the number of pixels raised by depression filling.
	FilledPixels int
	// FlatPixels is the number of pixels that had no downhill neighbor
	// after filling and were routed by flat resolution.
	FlatPixels int
}

// Downstream returns the pixel that pixel i drains into, or -1 if it
// drains off the map or is no-data.
func (h *Hydrology) Downstream(i int) int {
	d := h.Dir[i]
	if d < 1 || d > 8 {
		return -1
	}
	return i + nbrDY[d]*h.Width + nbrDX[d]
}

// Compute fills depressions in elev, in place, and computes flow directions
// and drainage area. Valid pixels on the raster edge or next to a no-data
// pixel drain off the map.
func Compute(width, height int, elev []int16) (*Hydrology, error) {
	if len(elev) != width*height {
		return nil, fmt.Errorf("elevation data has %d pixels, want %d", len(elev), width*height)
	}
	h := &Hydrology{Width: width, Height: height, Elev: elev, Dir: make([]uint8, len(elev))}
	h.markOutlets()
	h.fill()
	h.steepestDescent()
	if err := h.resolveFlats(); err != nil {
		return nil, err
	}
	if err := h.accumulate(); err != nil {
		return nil, err
	}
	return h, nil
}

// neighbor returns the index of pixel i's neighbor in direction d, or -1 if
// it is outside the raster.
func (h *Hydrology) neighbor(i int, d uint8) int {
	x, y := i%h.Width+nbrDX[d], i/h.Width+nbrDY[d]
	if x < 0 || y < 0 || x >= h.Width || y >= h.Height {
		return -1
	}
	return y*h.Width + x
}

func (h *Hydrology) valid(i int) bool {
	return i >= 0 && h.Elev[i] != dem2hm.NoDataPixel16
}

// markOutlets sets dirOut on valid pixels that touch the raster edge or a
// no-data pixel, dirFlat on every other valid pixel, and dirNone on no-data.
func (h *Hydrology) markOutlets() {
	for i, e := range h.Elev {
		if e == dem2hm.NoDataPixel16 {
			h.Dir[i] = dirNone
			continue
		}
		h.Dir[i] = dirFlat
		for d := uint8(1); d <= 8; d++ {
			if !h.valid(h.neighbor(i, d)) {
				h.Dir[i] = dirOut
				break
			}
		}
	}
}

// fill raises every closed depression to the level of its lowest spill
// point with a priority flood seeded from the outlets. Elevations are
// integers, so the priority queue is a FIFO bucket per meter.
func (h *Hydrology) fill() {
	lo, hi := int16(math.MaxInt16), int16(math.MinInt16)
	for _, e := range h.Elev {
		if e != dem2hm.NoDataPixel16 {
			lo, hi = min(lo, e), max(hi, e)
		}
	}
	if lo > hi {
		return
	}
	buckets := make([][]int32, int(hi)-int(lo)+1)
	visited := make([]bool, len(h.Elev))
	for i, d := range h.Dir {
		if d == dirOut {
			visited[i] = true
			b := int(h.Elev[i]) - int(lo)
			buckets[b] = append(buckets[b], int32(i))
		}
	}
	for b := 0; b < len(buckets); b++ {
		// Pixels pushed while draining bucket b go into bucket b or higher,
		// so iterate by index to see pixels appended to bucket b.
		for k := 0; k < len(buckets[b]); k++ {
			c := int(buckets[b][k])
			for d := uint8(1); d <= 8; d++ {
				n := h.neighbor(c, d)
				if !h.valid(n) || visited[n] {
					continue
				}
				visited[n] = true
				if h.Elev[n] < h.Elev[c] {
					h.Elev[n] = h.Elev[c]
					h.FilledPixels++
				}
				nb := int(h.Elev[n]) - int(lo)
				buckets[nb] = append(buckets[nb], int32(n))
			}
		}
		buckets[b] = nil
	}
}

// steepestDescent points each undecided pixel at its steepest downhill
// neighbor, allowing for diagonal neighbors being farther away. Pixels with
// no downhill neighbor stay dirFlat.
func (h *Hydrology) steepestDescent() {
	for i, dir := range h.Dir {
		if dir != dirFlat {
			continue
		}
		best, bestSlope := dirFlat, 0.0
		for d := uint8(1); d <= 8; d++ {
			n := h.neighbor(i, d)
			if !h.valid(n) || h.Elev[n] >= h.Elev[i] {
				continue
			}
			if s := float64(h.Elev[i]-h.Elev[n]) / nbrDist[d]; s > bestSlope {
				best, bestSlope = d, s
			}
		}
		h.Dir[i] = best
	}
}

// accumulate counts the pixels draining through each pixel by visiting
// pixels in upstream-to-downstream order.
func (h *Hydrology) accumulate() error {
	indeg := make([]uint8, len(h.Dir))
	h.Acc = make([]uint32, len(h.Dir))
	valid := 0
	for i := range h.Dir {
		if h.Dir[i] == dirNone {
			continue
		}
		valid++
		h.Acc[i] = 1
		if d := h.Downstream(i); d >= 0 {
			indeg[d]++
		}
	}
	var stack []int32
	for i, d := range h.Dir {
		if d != dirNone && indeg[i] == 0 {
			stack = append(stack, int32(i))
		}
	}
	done := 0
	for len(stack) > 0 {
		c := int(stack[len(stack)-1])
		stack = stack[:len(stack)-1]
		done++
		if d := h.Downstream(c); d >= 0 {
			h.Acc[d] += h.Acc[c]
			if indeg[d]--; indeg[d] == 0 {
				stack = append(stack, int32(d))
			}
		}
	}
	if done != valid {
		return fmt.Errorf("flow directions contain a cycle: %d of %d pixels drain off the map", done, valid)
	}
	return nil
}

// Trace follows the flow from pixel i to where it leaves the map and
// returns the last pixel and the number of steps.
func (h *Hydrology) Trace(i int) (last, steps int) {
	for {
		d := h.Downstream(i)
		if d < 0 {
			return i, steps
		}
		i, steps = d, steps+1
	}
}
