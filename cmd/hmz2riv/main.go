// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command hmz2riv extracts rivers from a dem2hm heightmap (format 1.1) at
// full resolution and snaps them to the edges of a flat-top hex grid,
// writing the river edges as JSON, with optional full-resolution river lines
// and a PNG preview.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/maloquacious/dem2hm"
	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "hmz2riv: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("hmz2riv", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hmz2riv [flags] <input.hmz>\n\n")
		fs.PrintDefaults()
	}
	apothem := fs.Int("apothem", 48, "hex apothem in raster pixels (a hex is twice this tall)")
	threshold := fs.Float64("threshold", 50, "drainage area, in km² of source ground, that makes a river")
	output := fs.String("output", "", "JSON file for the river edges (required)")
	lines := fs.String("lines", "", "JSON file for the full-resolution river lines (optional)")
	preview := fs.String("preview", "", "PNG preview file to write (optional)")
	previewScale := fs.Int("preview-scale", 8, "raster pixels per preview pixel, in each direction")
	previewLines := fs.Bool("preview-lines", false, "draw the full-resolution river lines under the snapped rivers")
	var barriers []hmz2riv.Barrier
	fs.Func("barrier", "wall `lon1,lat1,lon2,lat2,height` that water cannot cross (repeatable)", func(s string) error {
		b, err := hmz2riv.ParseBarrier(s)
		if err == nil {
			barriers = append(barriers, b)
		}
		return err
	})
	var traces [][2]float64
	fs.Func("trace", "print where water starting at `lon,lat` leaves the map (repeatable)", func(s string) error {
		lon, lat, ok := strings.Cut(s, ",")
		if !ok {
			return fmt.Errorf("want lon,lat")
		}
		x, err := strconv.ParseFloat(strings.TrimSpace(lon), 64)
		if err != nil {
			return err
		}
		y, err := strconv.ParseFloat(strings.TrimSpace(lat), 64)
		if err != nil {
			return err
		}
		traces = append(traces, [2]float64{x, y})
		return nil
	})
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, hmz2riv.Version())
		return nil
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected one input file, got %d", fs.NArg())
	}
	if *output == "" {
		return fmt.Errorf("-output is required")
	}
	if *threshold <= 0 {
		return fmt.Errorf("-threshold %g: must be positive", *threshold)
	}
	input := fs.Arg(0)
	start := time.Now()
	phase := func(name string) {
		fmt.Fprintf(stdout, "%-24s %6.1fs\n", name, time.Since(start).Seconds())
	}

	hm, err := readHeightMap(input)
	if err != nil {
		return err
	}
	phase("read heightmap")
	g, err := hmz2ele.NewGrid(*apothem, hm.Width, hm.Height)
	if err != nil {
		return err
	}
	pixelArea := hm.Metadata.PixelSizeMeters * hm.Metadata.PixelSizeMeters
	thresholdPixels := uint32(math.Ceil(*threshold * 1e6 / pixelArea))

	// Sample hex elevations for the preview before the heightmap is changed.
	var hexes []hmz2ele.Hex
	if *preview != "" {
		hexes = hmz2ele.Build(hmz2ele.RasterFromHeightMap(hm), g)
	}

	barrierPixels := 0
	var barrierNames []string
	for _, b := range barriers {
		n, err := b.Apply(hm)
		if err != nil {
			return err
		}
		barrierPixels += n
		barrierNames = append(barrierNames, b.String())
	}
	phase("apply barriers")

	h, err := hmz2riv.Compute(hm.Width, hm.Height, hm.Data)
	if err != nil {
		return err
	}
	phase("drainage")
	links := h.Rivers(thresholdPixels)
	phase("river links")
	edges, snapped := hmz2riv.Snap(h, g, links)
	phase("snap to hex edges")

	mouths := 0
	for _, l := range links {
		if l.Mouth {
			mouths++
		}
	}
	doc := hmz2riv.Rivers{
		Hmz2rivVersion: hmz2riv.Version().String(),
		Heightmap:      hmz2riv.HeightmapInfo{FileName: filepath.Base(input), Metadata: hm.Metadata},
		Grid:           hmz2riv.GridInfo{ApothemPx: g.Apothem, SidePx: g.Side, Columns: g.Columns, Rows: g.Rows},
		Hydrology: hmz2riv.HydrologyInfo{
			ThresholdKm2:    *threshold,
			PixelAreaM2:     pixelArea,
			ThresholdPixels: thresholdPixels,
			Barriers:        barrierNames,
			Method:          "priority-flood depression filling; D8 steepest descent; flats resolved toward outlets and away from higher ground (Barnes et al. 2014)",
		},
		Stats: hmz2riv.Stats{
			BarrierPixels: barrierPixels,
			FilledPixels:  h.FilledPixels,
			FlatPixels:    h.FlatPixels,
			Links:         len(links),
			Mouths:        mouths,
			RiverEdges:    len(edges),
		},
		Edges: hmz2riv.EdgesJSON(edges, pixelArea),
	}
	if err := writeJSON(*output, doc); err != nil {
		return err
	}
	if *lines != "" {
		if err := writeJSON(*lines, hmz2riv.LinesJSON(h, links, pixelArea)); err != nil {
			return err
		}
	}
	if *preview != "" {
		img, err := hmz2ele.RenderPreview(g, hexes, *previewScale)
		if err != nil {
			return err
		}
		if *previewLines {
			hmz2riv.DrawLinks(img, h, links, *previewScale)
		}
		hmz2riv.DrawEdges(img, g, edges, pixelArea, *previewScale)
		if err := writeFile(*preview, func(w io.Writer) error { return png.Encode(w, img) }); err != nil {
			return err
		}
	}
	phase("write output")

	hexKm := 10.0 / float64(2*g.Apothem) // campaign km per pixel, for a 10 km hex
	fmt.Fprintf(stdout, "threshold:          %g km² (%d pixels)\n", *threshold, thresholdPixels)
	fmt.Fprintf(stdout, "barrier pixels:     %d\n", barrierPixels)
	fmt.Fprintf(stdout, "filled pixels:      %d\n", h.FilledPixels)
	fmt.Fprintf(stdout, "flat pixels:        %d\n", h.FlatPixels)
	fmt.Fprintf(stdout, "river links:        %d (%d mouths)\n", len(links), mouths)
	fmt.Fprintf(stdout, "river edges:        %d (%.0f km of campaign river)\n", len(edges), float64(len(edges))*g.Side*hexKm)
	fmt.Fprintf(stdout, "single-vertex links: %d (%d mouths)\n", snapped.SingleVertexLinks, snapped.SingleVertexMouths)
	fmt.Fprintf(stdout, "extended links:     %d (%d edges)\n", snapped.Extended, snapped.ExtensionEdges)
	for _, t := range traces {
		x, y, err := hmz2riv.PixelFromLonLat(hm, t[0], t[1])
		if err != nil {
			return err
		}
		px, py := int(x), int(y)
		if px < 0 || py < 0 || px >= hm.Width || py >= hm.Height || hm.Data[py*hm.Width+px] == dem2hm.NoDataPixel16 {
			fmt.Fprintf(stdout, "trace %g,%g: not on land\n", t[0], t[1])
			continue
		}
		last, steps := h.Trace(py*hm.Width + px)
		lon, lat, err := hm.LonLat(last%hm.Width, last/hm.Width)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "trace %g,%g: pixel (%d, %d) flows %d steps to (%d, %d) = %.4f,%.4f\n",
			t[0], t[1], px, py, steps, last%hm.Width, last/hm.Width, lon, lat)
	}
	return nil
}

func readHeightMap(path string) (*dem2hm.HeightMap16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hm, err := dem2hm.ReadHeightMap16(bufio.NewReader(f))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return hm, nil
}

func writeJSON(path string, v any) error {
	return writeFile(path, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	})
}

func writeFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if err := write(w); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	return f.Close()
}
