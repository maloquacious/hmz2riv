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
	edges := Snap(hy, g, []Link{trib, trunk})
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
