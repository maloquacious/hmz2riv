// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/maloquacious/dem2hm"
)

// PixelFromLonLat returns the continuous raster coordinates of a longitude
// and latitude in the heightmap: the inverse of HeightMap16.LonLat.
func PixelFromLonLat(hm *dem2hm.HeightMap16, lon, lat float64) (x, y float64, err error) {
	src := hm.Metadata.Source
	g := src.GeoTransform
	det := g[1]*g[5] - g[2]*g[4]
	if det == 0 {
		return 0, 0, fmt.Errorf("geotransform %v is not invertible", g)
	}
	// Continuous source column u and row v.
	dlon, dlat := lon-g[0], lat-g[3]
	u := (g[5]*dlon - g[2]*dlat) / det
	v := (g[1]*dlat - g[4]*dlon) / det
	sw, sh := float64(src.Width), float64(src.Height)
	t := hm.Metadata.Transform
	switch t.Rotate {
	case 0:
		x, y = u, v
	case 90:
		x, y = sh-v, u
	case 180:
		x, y = sw-u, sh-v
	case 270:
		x, y = v, sw-u
	default:
		return 0, 0, fmt.Errorf("rotation %d is not supported", t.Rotate)
	}
	if t.FlipHorizontal {
		x = float64(hm.Width) - x
	}
	if t.FlipVertical {
		y = float64(hm.Height) - y
	}
	return x, y, nil
}

// Barrier is a wall along a line between two points, used to undo
// earthworks such as a canal cut. Valid pixels within barrierRadius of the
// line are raised to at least Height meters.
type Barrier struct {
	Lon1, Lat1, Lon2, Lat2 float64
	Height                 int16
}

// barrierRadius makes the wall at least two pixels thick in every direction,
// so water cannot slip between two raised pixels that touch only at a
// corner.
const barrierRadius = 1.5

// ParseBarrier parses "lon1,lat1,lon2,lat2,height".
func ParseBarrier(s string) (Barrier, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 5 {
		return Barrier{}, fmt.Errorf("barrier %q: want lon1,lat1,lon2,lat2,height", s)
	}
	var f [4]float64
	for i := range f {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil {
			return Barrier{}, fmt.Errorf("barrier %q: %w", s, err)
		}
		f[i] = v
	}
	height, err := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 16)
	if err != nil {
		return Barrier{}, fmt.Errorf("barrier %q: height: %w", s, err)
	}
	return Barrier{Lon1: f[0], Lat1: f[1], Lon2: f[2], Lat2: f[3], Height: int16(height)}, nil
}

func (b Barrier) String() string {
	return fmt.Sprintf("%g,%g,%g,%g,%d", b.Lon1, b.Lat1, b.Lon2, b.Lat2, b.Height)
}

// Apply raises the heightmap's valid pixels near the barrier's line and
// returns how many pixels it raised.
func (b Barrier) Apply(hm *dem2hm.HeightMap16) (int, error) {
	x1, y1, err := PixelFromLonLat(hm, b.Lon1, b.Lat1)
	if err != nil {
		return 0, err
	}
	x2, y2, err := PixelFromLonLat(hm, b.Lon2, b.Lat2)
	if err != nil {
		return 0, err
	}
	raised := 0
	minX := int(math.Floor(min(x1, x2) - barrierRadius))
	maxX := int(math.Ceil(max(x1, x2) + barrierRadius))
	minY := int(math.Floor(min(y1, y2) - barrierRadius))
	maxY := int(math.Ceil(max(y1, y2) + barrierRadius))
	for py := max(minY, 0); py <= min(maxY, hm.Height-1); py++ {
		for px := max(minX, 0); px <= min(maxX, hm.Width-1); px++ {
			if segmentDistance(float64(px)+0.5, float64(py)+0.5, x1, y1, x2, y2) > barrierRadius {
				continue
			}
			i := py*hm.Width + px
			if e := hm.Data[i]; e != dem2hm.NoDataPixel16 && e < b.Height {
				hm.Data[i] = b.Height
				raised++
			}
		}
	}
	return raised, nil
}

// segmentDistance returns the distance from (px, py) to the segment from
// (x1, y1) to (x2, y2).
func segmentDistance(px, py, x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1
	t := 0.0
	if l2 := dx*dx + dy*dy; l2 > 0 {
		t = max(0, min(1, ((px-x1)*dx+(py-y1)*dy)/l2))
	}
	return math.Hypot(px-(x1+t*dx), py-(y1+t*dy))
}
