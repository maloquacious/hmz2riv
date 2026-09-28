// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"testing"

	"github.com/maloquacious/dem2hm"
)

const nd = dem2hm.NoDataPixel16

// grid parses rows of elevations; "x" is no-data.
func grid(t *testing.T, rows ...[]int16) (int, int, []int16) {
	t.Helper()
	w := len(rows[0])
	var data []int16
	for _, r := range rows {
		if len(r) != w {
			t.Fatal("ragged grid")
		}
		data = append(data, r...)
	}
	return w, len(rows), data
}

func TestFillRaisesPitToSpillPoint(t *testing.T) {
	w, h, data := grid(t,
		[]int16{9, 9, 9, 9, 9},
		[]int16{9, 5, 5, 5, 9},
		[]int16{9, 5, 1, 5, 4},
		[]int16{9, 5, 5, 5, 9},
		[]int16{9, 9, 9, 9, 9},
	)
	hy, err := Compute(w, h, data)
	if err != nil {
		t.Fatal(err)
	}
	// The pit fills to the lowest spill point, 5, through the rim.
	if got := hy.Elev[2*w+2]; got != 5 {
		t.Errorf("pit filled to %d, want 5", got)
	}
	if hy.FilledPixels != 1 {
		t.Errorf("filled %d pixels, want 1", hy.FilledPixels)
	}
	// Everything drains, and the whole interior leaves through (4, 2).
	last, _ := hy.Trace(2*w + 2)
	if last != 2*w+3 && last != 2*w+4 {
		// The pit drains east toward the low outlet at (4, 2).
		x, y := last%w, last/w
		if x < 3 {
			t.Errorf("pit drains out at (%d, %d), want the east side", x, y)
		}
	}
}

func TestFlatDrainsTowardOutletAndAccumulates(t *testing.T) {
	// A flat valley at 3, walled by 9, with one outlet at the bottom middle.
	w, h, data := grid(t,
		[]int16{9, 9, 9, 9, 9, 9, 9},
		[]int16{9, 3, 3, 3, 3, 3, 9},
		[]int16{9, 3, 3, 3, 3, 3, 9},
		[]int16{9, 3, 3, 3, 3, 3, 9},
		[]int16{9, 3, 3, 3, 3, 3, 9},
		[]int16{9, 9, 9, 2, 9, 9, 9},
	)
	hy, err := Compute(w, h, data)
	if err != nil {
		t.Fatal(err)
	}
	if hy.FlatPixels == 0 {
		t.Fatal("expected flat pixels")
	}
	outlet := 5*w + 3
	for y := 1; y <= 4; y++ {
		for x := 1; x <= 5; x++ {
			if last, _ := hy.Trace(y*w + x); last != outlet && last != 4*w+3 {
				// Everything in the flat must leave through the outlet.
				if d := hy.Downstream(last); d != -1 || last/w != 5 {
					t.Errorf("(%d, %d) leaves the map at (%d, %d)", x, y, last%w, last/w)
				}
			}
		}
	}
	// The outlet collects the whole flat plus itself.
	if got := hy.Acc[outlet]; got < 21 {
		t.Errorf("outlet drains %d pixels, want at least 21", got)
	}
}

func TestAccumulationCountsEveryPixel(t *testing.T) {
	// A plane tilted toward a no-data sea on the right.
	w, h := 6, 4
	data := make([]int16, w*h)
	for y := range h {
		for x := range w {
			data[y*w+x] = int16(20 - 3*x + y%2)
		}
		data[y*w+w-1] = nd
	}
	hy, err := Compute(w, h, data)
	if err != nil {
		t.Fatal(err)
	}
	total := uint32(0)
	for i, d := range hy.Dir {
		if d == dirOut {
			total += hy.Acc[i]
		}
	}
	if total != uint32((w-1)*h) {
		t.Errorf("outlets drain %d pixels, want %d", total, (w-1)*h)
	}
}

func TestRiversLinkStructure(t *testing.T) {
	// Two valleys joining into one that runs to the sea at the bottom.
	//   columns 1 and 5 are tributaries, joining at (3, 4); column 3 below.
	w, h, data := grid(t,
		[]int16{50, 40, 50, 60, 50, 40, 50},
		[]int16{50, 35, 50, 60, 50, 35, 50},
		[]int16{50, 50, 30, 60, 30, 50, 50},
		[]int16{50, 50, 50, 25, 50, 50, 50},
		[]int16{50, 50, 50, 20, 50, 50, 50},
		[]int16{nd, nd, nd, nd, nd, nd, nd},
	)
	hy, err := Compute(w, h, data)
	if err != nil {
		t.Fatal(err)
	}
	links := hy.Rivers(3)
	var mouths, sources int
	for _, l := range links {
		if l.Mouth {
			mouths++
		}
		if hy.Acc[l.Pixels[0]] == 3 {
			sources++
		}
		for k := 0; k+1 < len(l.Pixels); k++ {
			if hy.Downstream(int(l.Pixels[k])) != int(l.Pixels[k+1]) {
				t.Errorf("link %v is not a flow path", l.Pixels)
			}
		}
	}
	if mouths != 1 {
		t.Errorf("%d links reach the sea, want 1", mouths)
	}
	if len(links) != 3 {
		t.Errorf("%d links, want 3 (two tributaries and the trunk)", len(links))
	}
}
