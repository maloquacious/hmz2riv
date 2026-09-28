# hmz2riv

`hmz2riv` extracts rivers from a `dem2hm` heightmap (format 1.1) at full resolution and snaps them to the edges of the flat-top hex grid used by [`hmz2ele`](https://github.com/maloquacious/hmz2ele).
It writes the river edges as JSON, with optional full-resolution river lines and a PNG preview.

## Usage

```text
go run ./cmd/hmz2riv [flags] <input.hmz>
```

Flags:

- `-apothem <px>` sets the hex apothem in raster pixels. The default is `48`.
- `-threshold <km²>` sets the drainage area, in km² of source ground, that makes a river. The default is `50`.
- `-barrier <lon1,lat1,lon2,lat2,height>` adds a wall that water cannot cross. Repeatable. See [Barriers](#barriers).
- `-output <file>` is the JSON file for the river edges. Required.
- `-lines <file>` is a JSON file for the full-resolution river lines. Optional.
- `-preview <file>` is a PNG preview to write: the `hmz2ele` hex preview with rivers drawn along hex edges. Optional.
- `-preview-scale <n>` sets how many raster pixels, in each direction, one preview pixel covers. The default is `8`.
- `-preview-lines` also draws the full-resolution river lines, in light blue, under the snapped rivers.
- `-trace <lon,lat>` prints where water starting at that point leaves the map. Repeatable. Useful for checking barriers.
- `-version` prints the version.

The command prints the time taken by each phase and a summary.
A full run on the Panama heightmap (189 million pixels) takes about 14 seconds and 4.3 GB of memory.

## Method

1. **Barriers.** Each barrier raises the valid pixels within 1.5 pixels of its line to at least its height.
2. **Depression filling.** A priority flood, seeded from every valid pixel on the raster edge or next to a no-data pixel, raises each closed basin to its lowest spill point. Elevations are whole meters, so the priority queue is one FIFO bucket per meter.
3. **Flow directions.** Pixels on the raster edge or next to no-data drain off the map. Every other pixel drains to its steepest downhill neighbor among its 8 neighbors, allowing for diagonal neighbors being √2 times farther away.
4. **Flats.** Pixels with no downhill neighbor are routed with the method of Barnes, Lehman, and Mulla (2014), "An efficient assignment of drainage direction over flat surfaces in raster digital elevation models": within each flat, water flows toward the flat's outlets and away from the higher ground around it.
5. **Drainage area.** Each pixel's drainage area is the number of pixels that drain through it, itself included, times the heightmap's pixel area (`pixel_size_m` squared).
6. **River links.** Pixels whose drainage area meets the threshold are river pixels. They are split into links, each running from a source or a confluence down to the next confluence or to where the river leaves the map.
7. **Snapping.** Links are snapped in decreasing order of drainage area at their downstream end, so each river is placed before its tributaries. Each pixel moves to its nearest hex vertex; gaps between consecutive vertices are filled with the shortest path along hex edges, and loops are cut out. A link stops at the first vertex already in the network, which is where it joins the river it flows into.

The result is a network in which every vertex has at most one outflow and every path leads to a river mouth.

## Barriers

A barrier undoes earthworks that would otherwise shape the drainage.
For the Panama heightmap, this barrier dams the Culebra Cut of the Panama Canal at its highest point, restoring the continental divide:

```text
-barrier=-79.6323,9.0652,-79.6599,9.0412,100
```

The line is about 4 km long and ends on ground above 120 m on both banks; 100 m is well above the lake levels on either side (about 26 m).
Without it, the divide still exists, but falls wherever flat resolution happens to split the canal's water surface, about 2 km south of the saddle.

## Output

`drainage_km2` is in km² of source (real-world) ground.

```json
{
  "hmz2riv_version": "0.1.0",
  "heightmap": { "file_name": "pandemokh.hmz", "metadata": { ... } },
  "grid": { "apothem_px": 48, "side_px": 55.43, "columns": 106, "rows": 222 },
  "hydrology": { "threshold_km2": 50, "pixel_area_m2": 943.8, "threshold_pixels": 52977, "barriers": [ "..." ], "method": "..." },
  "stats": { "barrier_pixels": 291, "filled_pixels": 1728, "flat_pixels": 12506022, "links": 758, "mouths": 132, "river_edges": 4667 },
  "edges": [
    { "col": 91, "row": 9, "side": "ne", "from": { "col": 92, "row": 8, "corner": "west" }, "to": { "col": 91, "row": 9, "corner": "east" }, "drainage_km2": 252.5 },
    ...
  ]
}
```

Each edge is identified by the hex that owns it and the side (`n`, `ne`, or `se`), following the `hmz2ele` ownership rules; water flows from `from` to `to`.
Vertices are identified by the hex that owns them and the corner (`west` or `east`).
Edges are ordered by owner row, column, and side.

The optional lines file lists each link as raster pixel coordinates, upstream to downstream, with its drainage area at the downstream end and whether it ends at a river mouth.

## License

MIT. See `LICENSE`.
