// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"math"
	"testing"

	"github.com/maloquacious/dem2hm"
)

func heightMap(width, height, rotate int, flipH, flipV bool, data []int16) *dem2hm.HeightMap16 {
	sw, sh := width, height
	if rotate == 90 || rotate == 270 {
		sw, sh = height, width
	}
	return &dem2hm.HeightMap16{
		Width: width, Height: height, Data: data,
		Metadata: dem2hm.Metadata{
			Width: int32(width), Height: int32(height),
			Source: dem2hm.SourceMetadata{
				Width: int32(sw), Height: int32(sh),
				GeoTransform: [6]float64{-80, 0.001, 0, 9, 0, -0.001},
			},
			Transform: dem2hm.TransformMetadata{Rotate: rotate, FlipHorizontal: flipH, FlipVertical: flipV},
		},
	}
}

func TestPixelFromLonLatInvertsLonLat(t *testing.T) {
	for _, rot := range []int{0, 90, 180, 270} {
		for _, flip := range [][2]bool{{false, false}, {true, false}, {false, true}} {
			w, h := 7, 5
			hm := heightMap(w, h, rot, flip[0], flip[1], make([]int16, w*h))
			for y := range h {
				for x := range w {
					lon, lat, err := hm.LonLat(x, y)
					if err != nil {
						t.Fatal(err)
					}
					px, py, err := PixelFromLonLat(hm, lon, lat)
					if err != nil {
						t.Fatal(err)
					}
					if math.Abs(px-(float64(x)+0.5)) > 1e-6 || math.Abs(py-(float64(y)+0.5)) > 1e-6 {
						t.Errorf("rotate %d flips %v: pixel (%d, %d) -> (%v, %v)", rot, flip, x, y, px, py)
					}
				}
			}
		}
	}
}

func TestBarrierBlocksDiagonalChannel(t *testing.T) {
	// A diagonal channel at 10 m from the top-left to the bottom-right
	// corner, in terrain at 100 m, with the sea off the bottom-right.
	w, h := 20, 20
	data := make([]int16, w*h)
	for i := range data {
		data[i] = 100
	}
	for k := range w {
		data[k*w+k] = int16(30 - k)
	}
	data[(h-1)*w+w-1] = nd
	hm := heightMap(w, h, 0, false, false, data)
	// Without a barrier, the channel's head drains to the sea corner.
	before := append([]int16(nil), data...)
	hy, err := Compute(w, h, before)
	if err != nil {
		t.Fatal(err)
	}
	if last, _ := hy.Trace(2*w + 2); last != (h-2)*w+w-2 {
		t.Fatalf("without barrier, channel ends at (%d, %d)", last%w, last/w)
	}
	// A wall across the middle of the channel, perpendicular to it.
	lon1, lat1, _ := hm.LonLat(14, 6)
	lon2, lat2, _ := hm.LonLat(6, 14)
	b := Barrier{Lon1: lon1, Lat1: lat1, Lon2: lon2, Lat2: lat2, Height: 200}
	raised, err := b.Apply(hm)
	if err != nil {
		t.Fatal(err)
	}
	if raised == 0 {
		t.Fatal("barrier raised no pixels")
	}
	hy, err = Compute(w, h, hm.Data)
	if err != nil {
		t.Fatal(err)
	}
	last, _ := hy.Trace(2*w + 2)
	if x, y := last%w, last/w; x+y > 19 {
		t.Errorf("channel head still drains past the barrier, to (%d, %d)", x, y)
	}
}
