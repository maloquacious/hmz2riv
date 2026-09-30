// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"testing"

	"github.com/maloquacious/hmz2ele"
)

func TestSnapBuildsConnectedDownstreamNetwork(t *testing.T) {
	// A wavy trunk river flowing right to the sea, with one tributary.
	w, h := 400, 200
	hy := &Hydrology{Width: w, Height: h, Dir: make([]uint8, w*h), Acc: make([]uint32, w*h)}
	var trunk, trib Link
	for x := 5; x < w-5; x++ {
		y := 100 + int(20*float64(x%80)/80)
		trunk.Pixels = append(trunk.Pixels, int32(y*w+x))
		hy.Acc[y*w+x] = uint32(1000 + x)
	}
	trunk.Mouth = true
	join := trunk.Pixels[200]
	jx, jy := int(join)%w, int(join)/w
	for y := 20; y < jy; y++ {
		trib.Pixels = append(trib.Pixels, int32(y*w+jx))
		hy.Acc[y*w+jx] = uint32(y)
	}
	trib.Pixels = append(trib.Pixels, join)
	g, err := hmz2ele.NewGrid(6, w, h)
	if err != nil {
		t.Fatal(err)
	}
	edges, _ := Snap(hy, g, []Link{trib, trunk})
	if len(edges) == 0 {
		t.Fatal("no edges")
	}
	// Every edge's endpoints are joined by that edge, each vertex has at
	// most one outflow, and following outflows from any vertex ends at the
	// trunk's last vertex.
	out := map[hmz2ele.VertexKey]hmz2ele.VertexKey{}
	for _, e := range edges {
		if got, ok := hmz2ele.EdgeBetween(e.From, e.To); !ok || got != e.Edge {
			t.Errorf("edge %v does not join %v and %v", e.Edge, e.From, e.To)
		}
		if prev, dup := out[e.From]; dup {
			t.Errorf("vertex %v flows to both %v and %v", e.From, prev, e.To)
		}
		out[e.From] = e.To
	}
	mouth := g.NearestVertex(float64(int(trunk.Pixels[len(trunk.Pixels)-1])%w)+0.5, float64(int(trunk.Pixels[len(trunk.Pixels)-1])/w)+0.5)
	for v := range out {
		u := v
		for steps := 0; ; steps++ {
			next, ok := out[u]
			if !ok {
				break
			}
			if steps > len(edges) {
				t.Fatalf("cycle from %v", v)
			}
			u = next
		}
		if u != mouth {
			t.Errorf("flow from %v ends at %v, want the mouth %v", v, u, mouth)
		}
	}
}

// TestSnapShortMouth checks that a river flowing into a mouth link too
// short to leave its vertex ends at that vertex, rather than being routed
// to the nearest other river.
func TestSnapShortMouth(t *testing.T) {
	w, h := 400, 200
	hy := &Hydrology{Width: w, Height: h, Dir: make([]uint8, w*h), Acc: make([]uint32, w*h)}
	px := func(x, y int, acc uint32) int32 {
		hy.Acc[y*w+x] = acc
		return int32(y*w + x)
	}
	g, err := hmz2ele.NewGrid(6, w, h)
	if err != nil {
		t.Fatal(err)
	}
	// The mouth link's two pixels, (xc, 60) and (xc+1, 60), snap to one
	// vertex.
	xc := 370
	for g.NearestVertex(float64(xc)+0.5, 60.5) != g.NearestVertex(float64(xc)+1.5, 60.5) {
		xc++
	}
	end := g.NearestVertex(float64(xc)+0.5, 60.5)

	// Another river, 40 px south, reaching the sea at the right.
	var other Link
	for x := 5; x < w-5; x++ {
		other.Pixels = append(other.Pixels, px(x, 100, uint32(5000+x)))
	}
	other.Mouth = true
	// A river flowing into a two-pixel mouth link.
	var river, mouth Link
	for x := 5; x <= xc; x++ {
		river.Pixels = append(river.Pixels, px(x, 60, uint32(x)))
	}
	mouth.Pixels = []int32{river.Pixels[len(river.Pixels)-1], px(xc+1, 60, 3000)}
	mouth.Mouth = true

	edges, rep := Snap(hy, g, []Link{river, mouth, other})
	if rep.SingleVertexMouths != 1 || rep.Extended != 0 {
		t.Errorf("report %+v; want 1 single-vertex mouth, 0 extended", rep)
	}
	out := map[hmz2ele.VertexKey]hmz2ele.VertexKey{}
	for _, e := range edges {
		out[e.From] = e.To
	}
	start := g.NearestVertex(5.5, 60.5)
	u := start
	for steps := 0; ; steps++ {
		next, ok := out[u]
		if !ok {
			break
		}
		if steps > len(edges) {
			t.Fatalf("cycle from %v", start)
		}
		u = next
	}
	if u != end {
		t.Errorf("river ends at %v, want the short mouth's vertex %v", u, end)
	}
}
