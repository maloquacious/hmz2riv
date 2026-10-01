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

The command prints the time taken by each phase and a summary, including the links that moved to a single vertex and the links extended to reach the network.
A full run on the Panama heightmap (189 million pixels) takes about 14 seconds and 4.3 GB of memory.

## Method

Pixel `(x, y)` has index `y·width + x`; "row-major order" means by increasing index.
A pixel's 8 neighbors are taken in **neighbor order**: east, south-east, south, south-west, west, north-west, north, north-east (clockwise from east, y down).
A pixel is **valid** if it isn't no-data; "next to no-data" means one of its 8 neighbors is no-data or off the raster.

1. **Barriers.** Each barrier raises every valid pixel whose center is at most 1.5 pixels from the line segment between its endpoints, and is below its height, to its height. The endpoints are converted to continuous raster coordinates by inverting the heightmap's `source.geotransform` and applying its `transform` (rotation and flips). `barrier_pixels` counts the pixels raised.
2. **Depression filling.** A priority flood, seeded from every valid pixel on the raster edge or next to no-data, raises each closed basin to its lowest spill point, flooding through 8-neighbors. `filled_pixels` counts the pixels raised. Elevations are whole meters, so the priority queue is one FIFO bucket per meter.
3. **Flow directions.** Pixels on the raster edge or next to no-data drain off the map. Every other pixel drains to its steepest downhill neighbor: the largest drop divided by the distance (1, or √2 for a diagonal). **Ties go to the first neighbor in neighbor order.**
4. **Flats.** Pixels that don't drain off the map and have no downhill neighbor (`flat_pixels`) are routed with the method of Barnes, Lehman, and Mulla (2014), "An efficient assignment of drainage direction over flat surfaces in raster digital elevation models": within each flat, water flows toward the flat's outlets and away from the higher ground around it, the pull toward the outlets weighted twice as much. Each flat pixel drains to the neighbor in its flat (outlets included) with the lowest combined value, if lower than its own; **ties go to the first neighbor in neighbor order**, with no preference for east, south, west, or north.
5. **Drainage area.** Each pixel's drainage area is the number of pixels that drain through it, itself included, times the heightmap's pixel area (`pixel_size_m` squared).
6. **River links.** A pixel is a river pixel if its drainage area, in pixels, is at least `threshold_pixels` = ⌈threshold · 10⁶ / pixel area⌉ (the threshold is in km², the pixel area in m²). Links start at every river pixel into which zero river pixels drain (a source) or two or more do (a confluence), in row-major order. Each runs downstream to the next confluence, included, or to the pixel that drains off the map, which makes it a **mouth** link. Links are numbered, and written to the lines file, in that order.
7. **Snapping.** Links are placed one at a time, in decreasing order of drainage area at their downstream end; ties, such as the two tributaries that end at the same confluence pixel, keep link order. Since drainage grows downstream, every link is placed after the link it flows into.
   - **Vertices.** Each pixel moves to the hex vertex nearest its **center** (`x + 0.5`, `y + 0.5`): the nearest of the six corners of the hex containing it, with ties going to the first corner in the order east, south-east, south-west, west, north-west, north-east.
   - **Gaps.** When consecutive pixels move to vertices that aren't joined by an edge, the gap is filled with the shortest path along hex edges, found by breadth-first search. The search expands vertices in the order it reaches them, and a vertex's neighbors in this order: from an `east` vertex (one identified by its owner's `east` corner), the lower-left, the upper-left, then the right neighbor; from a `west` vertex, the upper-right, the lower-right, then the left neighbor. A vertex's parent is the first vertex to reach it. If the gap is more than 16 edges long, the link is cut off at the gap.
   - **Loops.** When a link returns to a vertex already on its own path, the part of the path after that vertex is cut out.
   - **Order.** A link's pixels, and the vertices inserted to fill gaps, are processed one at a time from upstream to downstream, applying the loop and joining rules as each vertex is reached.
   - **Joining.** The **network** is the set of vertices of the links placed so far whose paths have at least one edge, plus the vertex of every mouth link whose path is a single vertex. A link stops at the first pixel (or gap vertex) that reaches a vertex in the network, which is where it joins the river it flows into; later pixels are ignored. A mouth link otherwise ends at its last vertex.
   - **Short links.** A link that isn't a mouth and whose path ends outside the network (because a gap cut it off, or the link below it left no vertex in the network) is continued along the shortest hex-edge path, searched as for gaps, to the nearest network vertex, if one is within 16 edges. The command reports these as "extended links"; the Panama run has none.
   - **Single-vertex links.** A link whose path is still a single vertex (its pixels all move to one vertex, or its first vertex is already in the network) places no edge. If it's a mouth, its vertex joins the network, so a river flowing into it ends there, on the shore. Otherwise it adds nothing to the network.
   - **Drainage.** Each vertex carries the largest drainage area of the pixels that moved to it before the link stopped (for a loop's end vertex, including the loop's pixels), so the joining vertex carries the drainage of the one pixel that reached it; a vertex inserted to fill a gap carries the drainage of the pixel after the gap, and a vertex added to reach the network carries that of the link's last vertex. An edge carries the drainage of its downstream vertex, `to`. If a later link's path runs along an edge already placed, the edge keeps its first placement.

The result is a network in which every vertex has at most one outflow and every path leads to a river mouth: the vertex of a mouth link's last pixel.

## River density

The default threshold, 50 km² of source ground, gives the Panama campaign map a dense network: 4,604 river edges for about 9,980 land hexes, roughly one river edge for every two land hexes.
The campaign starts with this setting on purpose.
Most rivers run from the central mountains to the coasts, across the map's north–south axis, so overland travel along the isthmus crosses river after river.
That slows north–south movement and pushes players to invest in ships, as travel along the isthmus did historically.
A larger threshold thins the network, keeping only the bigger rivers, if play shows it is too dense.

Rivers cross lakes in a straight line, because a lake's surface is one flat and flat resolution routes water straight toward the outlet.
Gatun Lake shows this.
It is left as is for now; marking lakes is a later step.

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
  "hmz2riv_version": "0.3.0",
  "heightmap": { "file_name": "pandemokh.hmz", "metadata": { ... } },
  "grid": { "apothem_px": 48, "side_px": 55.43, "columns": 106, "rows": 222 },
  "hydrology": { "threshold_km2": 50, "pixel_area_m2": 943.8, "threshold_pixels": 52977, "barriers": [ "..." ], "method": "..." },
  "stats": { "barrier_pixels": 291, "filled_pixels": 1728, "flat_pixels": 12506022, "links": 758, "mouths": 132, "river_edges": 4604 },
  "edges": [
    { "col": 91, "row": 9, "side": "ne", "from": { "col": 92, "row": 9, "corner": "west" }, "to": { "col": 91, "row": 9, "corner": "east" }, "drainage_km2": 478.3 },
    ...
  ]
}
```

Each edge is identified by the hex that owns it and the side (`n`, `ne`, or `se`), following the `hmz2ele` ownership rules; water flows from `from` to `to`.
Vertices are identified by the hex that owns them and the corner (`west` or `east`).
Edges are ordered by owner row, column, and side.

The optional lines file lists each link, in link order, as raster pixel coordinates, upstream to downstream, with its drainage area at the downstream end and whether it ends at a river mouth.

## License

MIT. See `LICENSE`.
