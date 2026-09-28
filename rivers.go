// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

// Link is one stretch of river: from a source or a confluence down to the
// next confluence, or to where the river leaves the map. Consecutive links
// share the confluence pixel.
type Link struct {
	// Pixels lists raster pixel indexes from upstream to downstream.
	Pixels []int32
	// Mouth reports whether the link ends where the river leaves the map.
	Mouth bool
}

// Rivers returns the links of every river: pixels whose drainage area is at
// least threshold pixels.
func (h *Hydrology) Rivers(threshold uint32) []Link {
	isRiver := func(i int) bool { return h.Acc[i] >= threshold }
	// Count river pixels flowing into each river pixel.
	upstream := make(map[int32]uint8)
	for i := range h.Dir {
		if h.Dir[i] == dirNone || !isRiver(i) {
			continue
		}
		if d := h.Downstream(i); d >= 0 {
			upstream[int32(d)]++
		}
	}
	var links []Link
	for i := range h.Dir {
		if h.Dir[i] == dirNone || !isRiver(i) {
			continue
		}
		// Links start at sources and at confluences.
		if up := upstream[int32(i)]; up == 1 {
			continue
		}
		l := Link{Pixels: []int32{int32(i)}}
		for c := i; ; {
			d := h.Downstream(c)
			if d < 0 {
				l.Mouth = true
				break
			}
			l.Pixels = append(l.Pixels, int32(d))
			if upstream[int32(d)] >= 2 {
				break
			}
			c = d
		}
		links = append(links, l)
	}
	return links
}
