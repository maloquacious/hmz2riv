// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"image"
	"image/color"
	"math"

	"github.com/maloquacious/hmz2ele"
)

var (
	riverColor = color.RGBA{0x10, 0x3a, 0xc8, 0xff}
	lineColor  = color.RGBA{0x00, 0xd0, 0xff, 0xff}
)

// DrawLinks draws the full-resolution links onto a preview, one preview
// pixel per covered cell.
func DrawLinks(img *image.RGBA, h *Hydrology, links []Link, scale int) {
	for _, l := range links {
		for _, p := range l.Pixels {
			img.SetRGBA(int(p)%h.Width/scale, int(p)/h.Width/scale, lineColor)
		}
	}
}

// DrawEdges draws the snapped river edges onto a preview, wider for larger
// drainage areas.
func DrawEdges(img *image.RGBA, g hmz2ele.Grid, edges []RiverEdge, pixelAreaM2 float64, scale int) {
	for _, e := range edges {
		x1, y1 := g.Vertex(e.From)
		x2, y2 := g.Vertex(e.To)
		r := 0
		switch a := km2(e.Acc, pixelAreaM2); {
		case a >= 5000:
			r = 2
		case a >= 500:
			r = 1
		}
		drawLine(img, x1/float64(scale), y1/float64(scale), x2/float64(scale), y2/float64(scale), r, riverColor)
	}
}

// drawLine stamps a (2r+1)-pixel square along the line.
func drawLine(img *image.RGBA, x1, y1, x2, y2 float64, r int, c color.RGBA) {
	steps := int(math.Ceil(max(math.Abs(x2-x1), math.Abs(y2-y1)))) + 1
	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps)
		cx, cy := int(x1+t*(x2-x1)), int(y1+t*(y2-y1))
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if (image.Point{cx + dx, cy + dy}).In(img.Rect) {
					img.SetRGBA(cx+dx, cy+dy, c)
				}
			}
		}
	}
}
