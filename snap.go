// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"cmp"
	"slices"

	"github.com/maloquacious/hmz2ele"
)

// RiverEdge is one hex edge carrying a river, with the direction of flow
// and the drainage area, in pixels, where the river leaves the edge.
type RiverEdge struct {
	Edge     hmz2ele.EdgeKey
	From, To hmz2ele.VertexKey
	Acc      uint32
}

// SnapReport counts how snapping placed the links.
type SnapReport struct {
	// SingleVertexLinks counts the links whose pixels all snap to one
	// vertex, so they place no edge; SingleVertexMouths counts those that
	// are mouths, whose vertex joins the network as a river mouth.
	SingleVertexLinks, SingleVertexMouths int
	// Extended counts the links continued to the nearest network vertex
	// because they ended short of the network, and ExtensionEdges the edges
	// that added.
	Extended, ExtensionEdges int
}

// maxGap is the most edges snapping will insert between two consecutive
// river vertices. Consecutive pixels are never more than a few vertices
// apart, so a longer gap means something is wrong and the link is cut.
const maxGap = 16

// Snap moves each link onto a chain of hex edges and returns the edges of
// the resulting river network, ordered by owner row, column, and side.
//
// Links are snapped in decreasing order of drainage area at their
// downstream end, so every river is placed before its tributaries. Each
// river pixel moves to its nearest vertex; gaps between consecutive
// vertices are filled with the shortest path along hex edges, and loops are
// cut out. A link stops at the first vertex already in the network, which
// is where it joins the river it flows into.
//
// A link whose pixels all snap to one vertex places no edge. Its vertex
// joins the network only if the link is a mouth, so a river flowing into a
// short mouth link ends at that vertex, on the shore.
func Snap(h *Hydrology, g hmz2ele.Grid, links []Link) ([]RiverEdge, SnapReport) {
	var rep SnapReport
	order := make([]int, len(links))
	for i := range order {
		order[i] = i
	}
	endAcc := func(l Link) uint32 { return h.Acc[l.Pixels[len(l.Pixels)-1]] }
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Compare(endAcc(links[b]), endAcc(links[a]))
	})

	inNetwork := map[hmz2ele.VertexKey]bool{}
	edges := map[hmz2ele.EdgeKey]RiverEdge{}
	for _, li := range order {
		l := links[li]
		path, accs := snapLink(h, g, l, inNetwork)
		if len(path) > 0 && !l.Mouth && !inNetwork[path[len(path)-1]] {
			// The link ended short of the river it flows into: a gap wider
			// than maxGap cut it off, or the link below it placed no vertex.
			// Connect to the nearest network vertex.
			if tail := pathTo(path[len(path)-1], func(v hmz2ele.VertexKey) bool { return inNetwork[v] }); tail != nil {
				rep.Extended++
				rep.ExtensionEdges += len(tail)
				for _, v := range tail {
					path = append(path, v)
					accs = append(accs, accs[len(accs)-1])
				}
			}
		}
		for k := 0; k+1 < len(path); k++ {
			e, ok := hmz2ele.EdgeBetween(path[k], path[k+1])
			if !ok {
				panic("snapped path has a gap")
			}
			if _, dup := edges[e]; !dup {
				edges[e] = RiverEdge{Edge: e, From: path[k], To: path[k+1], Acc: accs[k+1]}
			}
		}
		if len(path) == 1 {
			// Too short to leave its first vertex: no edges. A mouth's
			// vertex is where the river meets the sea, so rivers flowing
			// into it end there; any other link leaves no vertex to join.
			rep.SingleVertexLinks++
			if l.Mouth {
				rep.SingleVertexMouths++
				inNetwork[path[0]] = true
			}
			continue
		}
		if len(path) == 0 {
			continue
		}
		for _, v := range path {
			inNetwork[v] = true
		}
	}

	out := make([]RiverEdge, 0, len(edges))
	for _, e := range edges {
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b RiverEdge) int {
		return cmp.Or(
			cmp.Compare(a.Edge.Row, b.Edge.Row),
			cmp.Compare(a.Edge.Col, b.Edge.Col),
			cmp.Compare(a.Edge.Side, b.Edge.Side),
		)
	})
	return out, rep
}

// snapLink returns the link's vertex path and the largest drainage area
// seen at each vertex. The path stops early at a vertex in the network.
func snapLink(h *Hydrology, g hmz2ele.Grid, l Link, inNetwork map[hmz2ele.VertexKey]bool) ([]hmz2ele.VertexKey, []uint32) {
	var path []hmz2ele.VertexKey
	var accs []uint32
	index := map[hmz2ele.VertexKey]int{}
	joined := false
	add := func(v hmz2ele.VertexKey, acc uint32) {
		if at, seen := index[v]; seen {
			// Cut out the loop back to v.
			for _, cut := range path[at+1:] {
				delete(index, cut)
			}
			path, accs = path[:at+1], accs[:at+1]
			accs[at] = max(accs[at], acc)
			return
		}
		index[v] = len(path)
		path = append(path, v)
		accs = append(accs, acc)
		joined = inNetwork[v]
	}
	for _, p := range l.Pixels {
		acc := h.Acc[p]
		x, y := float64(int(p)%h.Width)+0.5, float64(int(p)/h.Width)+0.5
		v := g.NearestVertex(x, y)
		if n := len(path); n > 0 {
			last := path[n-1]
			if v == last {
				accs[n-1] = max(accs[n-1], acc)
				continue
			}
			if _, adjacent := hmz2ele.EdgeBetween(last, v); !adjacent {
				gap := pathTo(last, func(u hmz2ele.VertexKey) bool { return u == v })
				if gap == nil {
					break
				}
				for _, u := range gap[:len(gap)-1] {
					if add(u, acc); joined {
						return path, accs
					}
				}
			}
		}
		if add(v, acc); joined {
			return path, accs
		}
	}
	return path, accs
}

// pathTo returns the shortest path along hex edges from start to the
// nearest vertex satisfying goal, excluding start and including the goal,
// or nil if none is within maxGap edges.
func pathTo(start hmz2ele.VertexKey, goal func(hmz2ele.VertexKey) bool) []hmz2ele.VertexKey {
	parent := map[hmz2ele.VertexKey]hmz2ele.VertexKey{start: start}
	frontier := []hmz2ele.VertexKey{start}
	for depth := 0; depth < maxGap && len(frontier) > 0; depth++ {
		var next []hmz2ele.VertexKey
		for _, u := range frontier {
			for _, v := range hmz2ele.VertexNeighbors(u) {
				if _, seen := parent[v]; seen {
					continue
				}
				parent[v] = u
				if goal(v) {
					var rev []hmz2ele.VertexKey
					for w := v; w != start; w = parent[w] {
						rev = append(rev, w)
					}
					slices.Reverse(rev)
					return rev
				}
				next = append(next, v)
			}
		}
		frontier = next
	}
	return nil
}
