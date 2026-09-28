// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import "fmt"

// resolveFlats assigns a flow direction to every pixel left dirFlat by
// steepestDescent, following Barnes, Lehman, and Mulla (2014), "An
// efficient assignment of drainage direction over flat surfaces in raster
// digital elevation models". Within each flat, water flows toward the flat's
// outlets and away from the higher ground around it, with the pull toward
// the outlets weighted twice as much.
func (h *Hydrology) resolveFlats() error {
	n := len(h.Dir)
	var lowEdges, highEdges []int32
	for i, dir := range h.Dir {
		if dir == dirNone {
			continue
		}
		if dir == dirFlat {
			h.FlatPixels++
		}
		for d := uint8(1); d <= 8; d++ {
			nb := h.neighbor(i, d)
			if !h.valid(nb) {
				continue
			}
			if dir != dirFlat && h.Dir[nb] == dirFlat && h.Elev[nb] == h.Elev[i] {
				// A draining pixel at the level of a flat: the flat's outlet.
				lowEdges = append(lowEdges, int32(i))
				break
			}
			if dir == dirFlat && h.Elev[i] < h.Elev[nb] {
				highEdges = append(highEdges, int32(i))
				break
			}
		}
	}
	if h.FlatPixels == 0 {
		return nil
	}

	// Label each flat, flooding from its outlets through pixels at the same
	// elevation.
	labels := make([]int32, n)
	next := int32(1)
	var queue []int32
	for _, c := range lowEdges {
		if labels[c] != 0 {
			continue
		}
		labels[c] = next
		queue = append(queue[:0], c)
		for len(queue) > 0 {
			p := int(queue[len(queue)-1])
			queue = queue[:len(queue)-1]
			for d := uint8(1); d <= 8; d++ {
				nb := h.neighbor(p, d)
				if h.valid(nb) && labels[nb] == 0 && h.Elev[nb] == h.Elev[c] {
					labels[nb] = next
					queue = append(queue, int32(nb))
				}
			}
		}
		next++
	}

	// Distance from higher ground, by breadth-first search from the high
	// edges through each flat. flatHeight is the largest distance per flat.
	mask := make([]int32, n)
	flatHeight := make([]int32, next)
	frontier := highEdges[:0:0]
	for _, c := range highEdges {
		if labels[c] != 0 {
			frontier = append(frontier, c)
		}
	}
	for loops := int32(1); len(frontier) > 0; loops++ {
		var following []int32
		for _, c := range frontier {
			if mask[c] > 0 {
				continue
			}
			mask[c] = loops
			flatHeight[labels[c]] = loops
			for d := uint8(1); d <= 8; d++ {
				nb := h.neighbor(int(c), d)
				if nb >= 0 && labels[nb] == labels[c] && h.Dir[nb] == dirFlat && mask[nb] == 0 {
					following = append(following, int32(nb))
				}
			}
		}
		frontier = following
	}

	// Combine with distance from the outlets. A pixel far from the outlets
	// is "higher"; a pixel far from higher ground is "lower".
	for i := range mask {
		mask[i] = -mask[i]
	}
	frontier = lowEdges
	for loops := int32(1); len(frontier) > 0; loops++ {
		var following []int32
		for _, c := range frontier {
			if mask[c] > 0 {
				continue
			}
			if mask[c] < 0 {
				mask[c] = flatHeight[labels[c]] + mask[c] + 2*loops
			} else {
				mask[c] = 2 * loops
			}
			for d := uint8(1); d <= 8; d++ {
				nb := h.neighbor(int(c), d)
				if nb >= 0 && labels[nb] == labels[c] && h.Dir[nb] == dirFlat && mask[nb] <= 0 {
					following = append(following, int32(nb))
				}
			}
		}
		frontier = following
	}

	// Point each flat pixel at the neighbor in its flat with the lowest mask.
	unresolved := 0
	for i, dir := range h.Dir {
		if dir != dirFlat {
			continue
		}
		best, bestMask := dirFlat, mask[i]
		for d := uint8(1); d <= 8; d++ {
			nb := h.neighbor(i, d)
			if nb >= 0 && labels[nb] == labels[i] && labels[i] != 0 && mask[nb] < bestMask {
				best, bestMask = d, mask[nb]
			}
		}
		if best == dirFlat {
			unresolved++
		}
		h.Dir[i] = best
	}
	if unresolved > 0 {
		return fmt.Errorf("%d flat pixels have no drainage direction", unresolved)
	}
	return nil
}
