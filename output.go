// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"math"

	"github.com/maloquacious/dem2hm"
	"github.com/maloquacious/hmz2ele"
)

// Rivers is the river network snapped to hex edges, as written to JSON.
type Rivers struct {
	Hmz2rivVersion string        `json:"hmz2riv_version"`
	Heightmap      HeightmapInfo `json:"heightmap"`
	Grid           GridInfo      `json:"grid"`
	Hydrology      HydrologyInfo `json:"hydrology"`
	Stats          Stats         `json:"stats"`
	Edges          []EdgeJSON    `json:"edges"`
}

// HeightmapInfo identifies the heightmap the rivers were extracted from.
type HeightmapInfo struct {
	FileName string          `json:"file_name"`
	Metadata dem2hm.Metadata `json:"metadata"`
}

// GridInfo identifies the hex grid the rivers were snapped to.
type GridInfo struct {
	ApothemPx int     `json:"apothem_px"`
	SidePx    float64 `json:"side_px"`
	Columns   int     `json:"columns"`
	Rows      int     `json:"rows"`
}

// HydrologyInfo records the settings used to extract the rivers.
type HydrologyInfo struct {
	ThresholdKm2    float64  `json:"threshold_km2"`
	PixelAreaM2     float64  `json:"pixel_area_m2"`
	ThresholdPixels uint32   `json:"threshold_pixels"`
	Barriers        []string `json:"barriers"`
	Method          string   `json:"method"`
}

// Stats summarizes the extraction.
type Stats struct {
	BarrierPixels int `json:"barrier_pixels"`
	FilledPixels  int `json:"filled_pixels"`
	FlatPixels    int `json:"flat_pixels"`
	Links         int `json:"links"`
	Mouths        int `json:"mouths"`
	RiverEdges    int `json:"river_edges"`
}

// VertexJSON identifies a vertex by its owning hex and corner.
type VertexJSON struct {
	Col    int            `json:"col"`
	Row    int            `json:"row"`
	Corner hmz2ele.Corner `json:"corner"`
}

// EdgeJSON is one hex edge carrying a river. The edge is identified by the
// hex that owns it and the side; water flows from From to To. DrainageKm2 is
// the area of source ground draining through the edge.
type EdgeJSON struct {
	Col         int          `json:"col"`
	Row         int          `json:"row"`
	Side        hmz2ele.Side `json:"side"`
	From        VertexJSON   `json:"from"`
	To          VertexJSON   `json:"to"`
	DrainageKm2 float64      `json:"drainage_km2"`
}

// Lines holds the full-resolution river links, as written to JSON.
type Lines struct {
	Hmz2rivVersion string     `json:"hmz2riv_version"`
	Links          []LinkJSON `json:"links"`
}

// LinkJSON is one link as raster pixel coordinates, upstream to downstream.
type LinkJSON struct {
	Mouth       bool     `json:"mouth"`
	DrainageKm2 float64  `json:"drainage_km2"`
	Points      [][2]int `json:"points"`
}

func vertexJSON(k hmz2ele.VertexKey) VertexJSON {
	return VertexJSON{Col: k.Col, Row: k.Row, Corner: k.Corner}
}

// km2 converts a pixel count to km², rounded to 0.1 km².
func km2(pixels uint32, pixelAreaM2 float64) float64 {
	return math.Round(float64(pixels)*pixelAreaM2/1e5) / 10
}

// EdgesJSON converts snapped edges for output.
func EdgesJSON(edges []RiverEdge, pixelAreaM2 float64) []EdgeJSON {
	out := make([]EdgeJSON, len(edges))
	for i, e := range edges {
		out[i] = EdgeJSON{
			Col:         e.Edge.Col,
			Row:         e.Edge.Row,
			Side:        e.Edge.Side,
			From:        vertexJSON(e.From),
			To:          vertexJSON(e.To),
			DrainageKm2: km2(e.Acc, pixelAreaM2),
		}
	}
	return out
}

// LinesJSON converts links for output.
func LinesJSON(h *Hydrology, links []Link, pixelAreaM2 float64) Lines {
	out := Lines{Hmz2rivVersion: Version().String(), Links: make([]LinkJSON, len(links))}
	for i, l := range links {
		pts := make([][2]int, len(l.Pixels))
		for k, p := range l.Pixels {
			pts[k] = [2]int{int(p) % h.Width, int(p) / h.Width}
		}
		out.Links[i] = LinkJSON{
			Mouth:       l.Mouth,
			DrainageKm2: km2(h.Acc[l.Pixels[len(l.Pixels)-1]], pixelAreaM2),
			Points:      pts,
		}
	}
	return out
}
