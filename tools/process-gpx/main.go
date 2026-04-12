// Reads 12 WB GPX files, concatenates in reverse file order (12→1) for the
// eastbound Yorktown→Astoria direction, computes cumulative mileage,
// downsamples, and writes frontend/js/route-data.js and waypoints-data.js.
package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ── GPX XML types ─────────────────────────────────────────────────────────────

type gpxEle struct {
	Val float64 `xml:",chardata"`
}

type gpxTrkpt struct {
	Lat float64 `xml:"lat,attr"`
	Lon float64 `xml:"lon,attr"`
	Ele *gpxEle `xml:"ele"`
}

type gpxWpt struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Name string  `xml:"name"`
	Cmt  string  `xml:"cmt"`
	Desc string  `xml:"desc"`
}

type gpxFile struct {
	Trkpts []gpxTrkpt `xml:"trk>trkseg>trkpt"`
	Wpts   []gpxWpt   `xml:"wpt"`
}

// ── Point ─────────────────────────────────────────────────────────────────────

type point struct {
	lat, lng float64
	ele      float64
	hasEle   bool
	meters   float64
}

// ── Stop (waypoint POI) ───────────────────────────────────────────────────────

type stop struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Cat  string  `json:"cat"`
	Name string  `json:"name"`
	Desc string  `json:"desc,omitempty"`
}

// ── Town ──────────────────────────────────────────────────────────────────────

type town struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Name string  `json:"name"`
}

// cmtToCategory maps GPX <cmt> values to display categories.
var cmtToCategory = map[string]string{
	"camping":  "campsite",
	"lodging":  "lodging",
	"bike_shop": "bike_shop",
	"library":  "library",
	"gas":      "services",
	"food":     "food",
	"caution":  "caution",
}

// codeRe strips Adventure Cycling prefix codes like "CG,CBN-" or "POI - ".
var codeRe = regexp.MustCompile(`^[?A-Z,&\s]+-\s*(.+)$`)

func cleanName(raw string) string {
	raw = strings.TrimSpace(raw)
	if m := codeRe.FindStringSubmatch(raw); m != nil {
		return strings.TrimSpace(m[1])
	}
	return raw
}

// ── Haversine distance (meters) ───────────────────────────────────────────────

func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// ── Parse one GPX file ────────────────────────────────────────────────────────

// nsRe strips xmlns declarations so encoding/xml can match by local name.
var nsRe = regexp.MustCompile(`\s+xmlns(?::[a-z]+)?="[^"]*"`)

func parseGPX(path string) ([]point, []gpxWpt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	data = nsRe.ReplaceAll(data, nil)

	var g gpxFile
	if err := xml.Unmarshal(data, &g); err != nil {
		return nil, nil, err
	}
	pts := make([]point, len(g.Trkpts))
	for i, t := range g.Trkpts {
		pts[i] = point{lat: t.Lat, lng: t.Lon}
		if t.Ele != nil {
			pts[i].ele = t.Ele.Val * 3.28084 // metres → feet
			pts[i].hasEle = true
		}
	}
	return pts, g.Wpts, nil
}

// ── Section names (east→west order, file 12→1) ────────────────────────────────

var sectionNames = []string{
	"Virginia",            // 12
	"Virginia & Kentucky", // 11
	"Kentucky",            // 10
	"Illinois & Missouri", // 09
	"Missouri & Kansas",   // 08
	"Kansas",              // 07
	"Colorado",            // 06
	"Wyoming",             // 05
	"Montana & Idaho",     // 04
	"Montana",             // 03
	"Idaho & Oregon",      // 02
	"Oregon",              // 01
}

func metersToMiles(m float64) float64 { return m / 1609.344 }

func main() {
	gpxDir := os.Getenv("GPX_DIR")
	if gpxDir == "" {
		gpxDir = "./gpx"
	}
	outFile := os.Getenv("OUT_FILE")
	if outFile == "" {
		outFile = "./frontend/js/route-data.js"
	}
	stopsFile := strings.Replace(outFile, "route-data.js", "waypoints-data.js", 1)

	// ── Load GPX files in reverse sort order (12→1) ───────────────────────────
	entries, err := os.ReadDir(gpxDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading GPX dir: %v\n", err)
		os.Exit(1)
	}
	var gpxFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".gpx") {
			gpxFiles = append(gpxFiles, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(gpxFiles)))

	if len(gpxFiles) != 12 {
		fmt.Fprintf(os.Stderr, "Warning: expected 12 GPX files, found %d\n", len(gpxFiles))
	}
	fmt.Printf("Processing %d files:\n", len(gpxFiles))

	// ── Parse all files, track per-file point counts ──────────────────────────
	var rawTrack []point
	var rawWpts []gpxWpt
	fileLengths := make([]int, len(gpxFiles))

	for i, fname := range gpxFiles {
		pts, wpts, err := parseGPX(filepath.Join(gpxDir, fname))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing %s: %v\n", fname, err)
			os.Exit(1)
		}
		fileLengths[i] = len(pts)
		if len(rawTrack) == 0 {
			rawTrack = append(rawTrack, pts...)
		} else {
			// Drop the first point — it duplicates the last point of the previous file.
			rawTrack = append(rawTrack, pts[1:]...)
		}
		rawWpts = append(rawWpts, wpts...)
		fmt.Printf("  %s: %d track pts, %d waypoints\n", fname, len(pts), len(wpts))
	}
	fmt.Printf("Total raw points: %d\n", len(rawTrack))

	// ── Compute cumulative distances and section boundaries ───────────────────
	sectionBoundaryMeters := make([]float64, len(gpxFiles)+1)
	// sectionBoundaryMeters[0] = 0 (zero value)

	rawTrack[0].meters = 0
	cumMeters := 0.0
	fileIdx := 0
	pointsInCurrentFile := fileLengths[0]
	pointsConsumed := 0

	for i := 1; i < len(rawTrack); i++ {
		cumMeters += haversine(
			rawTrack[i-1].lat, rawTrack[i-1].lng,
			rawTrack[i].lat, rawTrack[i].lng,
		)
		rawTrack[i].meters = cumMeters

		pointsConsumed++
		if pointsConsumed >= pointsInCurrentFile && fileIdx+1 < len(fileLengths) {
			fileIdx++
			sectionBoundaryMeters[fileIdx] = cumMeters
			pointsInCurrentFile = fileLengths[fileIdx] - 1
			pointsConsumed = 0
		}
	}
	sectionBoundaryMeters[len(gpxFiles)] = cumMeters

	totalMiles := metersToMiles(cumMeters)
	fmt.Printf("Total distance: %.1f miles\n", totalMiles)

	// ── Downsample for map track and elevation profile ────────────────────────
	const trackIntervalM = 2000
	const elevIntervalM = 8000

	var trackPts, elevPts []point
	lastTrackM := math.Inf(-1)
	lastElevM := math.Inf(-1)

	for _, pt := range rawTrack {
		if pt.meters-lastTrackM >= trackIntervalM {
			trackPts = append(trackPts, pt)
			lastTrackM = pt.meters
		}
		if pt.meters-lastElevM >= elevIntervalM {
			elevPts = append(elevPts, pt)
			lastElevM = pt.meters
		}
	}
	last := rawTrack[len(rawTrack)-1]
	if trackPts[len(trackPts)-1].meters != last.meters {
		trackPts = append(trackPts, last)
	}
	if elevPts[len(elevPts)-1].meters != last.meters {
		elevPts = append(elevPts, last)
	}

	// Filter elevation points to those that have elevation data.
	var elevFiltered []point
	for _, p := range elevPts {
		if p.hasEle {
			elevFiltered = append(elevFiltered, p)
		}
	}

	fmt.Printf("Track points (map): %d\n", len(trackPts))
	fmt.Printf("Elevation points:   %d\n", len(elevFiltered))

	// ── Build section boundaries ──────────────────────────────────────────────
	type section struct {
		Name  string  `json:"name"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	}
	sections := make([]section, len(sectionNames))
	for i, name := range sectionNames {
		sections[i] = section{
			Name:  name,
			Start: math.Round(metersToMiles(sectionBoundaryMeters[i])*10) / 10,
			End:   math.Round(metersToMiles(sectionBoundaryMeters[i+1])*10) / 10,
		}
	}
	sectionsJSON, _ := json.MarshalIndent(sections, "", "  ")

	// ── Format track points ───────────────────────────────────────────────────
	var trackBuf strings.Builder
	trackBuf.WriteString("[\n")
	for i, p := range trackPts {
		if p.hasEle {
			fmt.Fprintf(&trackBuf, "  {lat:%.5f,lng:%.5f,mile:%.2f,ele:%d}",
				p.lat, p.lng, metersToMiles(p.meters), int(math.Round(p.ele)))
		} else {
			fmt.Fprintf(&trackBuf, "  {lat:%.5f,lng:%.5f,mile:%.2f}",
				p.lat, p.lng, metersToMiles(p.meters))
		}
		if i < len(trackPts)-1 {
			trackBuf.WriteByte(',')
		}
		trackBuf.WriteByte('\n')
	}
	trackBuf.WriteByte(']')

	// ── Format elevation points ───────────────────────────────────────────────
	var elevBuf strings.Builder
	elevBuf.WriteString("[\n")
	for i, p := range elevFiltered {
		fmt.Fprintf(&elevBuf, "  {mile:%.1f,ele:%d}",
			metersToMiles(p.meters), int(math.Round(p.ele)))
		if i < len(elevFiltered)-1 {
			elevBuf.WriteByte(',')
		}
		elevBuf.WriteByte('\n')
	}
	elevBuf.WriteByte(']')

	// ── Write route-data.js ───────────────────────────────────────────────────
	output := fmt.Sprintf(
		"// TransAmerica Trail — generated by tools/process-gpx/main.go\n"+
			"// Source: %d Adventure Cycling WB GPX files, sections 12→1 (Yorktown→Astoria)\n"+
			"// Raw points: %d · Map track: %d · Elev profile: %d\n"+
			"// DO NOT EDIT — regenerate with: make gpx\n"+
			"\n"+
			"const ROUTE_TOTAL_MILES = %.1f;\n"+
			"\n"+
			"// Dense track used for map polyline and mile→lat/lng interpolation\n"+
			"const ROUTE_WAYPOINTS = %s;\n"+
			"\n"+
			"// Sparse track used for the full-route elevation profile chart\n"+
			"const ROUTE_ELEVATION = %s;\n"+
			"\n"+
			"// Section boundaries (mile markers computed from GPS data)\n"+
			"const ROUTE_SECTIONS = %s;\n"+
			"\n"+
			"// Returns the section name for a given mile marker\n"+
			"function currentSection(mile) {\n"+
			"  for (const s of ROUTE_SECTIONS) {\n"+
			"    if (mile >= s.start && mile <= s.end) return s.name;\n"+
			"  }\n"+
			"  return '';\n"+
			"}\n"+
			"\n"+
			"// Interpolates a [lat, lng] position on the route for a given mile\n"+
			"function latLngForMile(mile) {\n"+
			"  if (mile <= 0)                 return [ROUTE_WAYPOINTS[0].lat,    ROUTE_WAYPOINTS[0].lng];\n"+
			"  if (mile >= ROUTE_TOTAL_MILES) return [ROUTE_WAYPOINTS.at(-1).lat, ROUTE_WAYPOINTS.at(-1).lng];\n"+
			"  for (let i = 1; i < ROUTE_WAYPOINTS.length; i++) {\n"+
			"    const prev = ROUTE_WAYPOINTS[i - 1];\n"+
			"    const curr = ROUTE_WAYPOINTS[i];\n"+
			"    if (mile <= curr.mile) {\n"+
			"      const t = (mile - prev.mile) / (curr.mile - prev.mile);\n"+
			"      return [\n"+
			"        prev.lat + t * (curr.lat - prev.lat),\n"+
			"        prev.lng + t * (curr.lng - prev.lng),\n"+
			"      ];\n"+
			"    }\n"+
			"  }\n"+
			"  return [ROUTE_WAYPOINTS.at(-1).lat, ROUTE_WAYPOINTS.at(-1).lng];\n"+
			"}\n",
		len(gpxFiles), len(rawTrack), len(trackPts), len(elevFiltered),
		totalMiles,
		trackBuf.String(),
		elevBuf.String(),
		string(sectionsJSON),
	)

	if err := os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outFile, []byte(output), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s (%d KB)\n", outFile, len(output)/1024)

	// ── Process waypoints → waypoints-data.js ────────────────────────────────
	seen := make(map[string]bool)
	var stops []stop

	for _, w := range rawWpts {
		cat, ok := cmtToCategory[w.Cmt]
		if !ok {
			continue // skip uncategorized types
		}
		name := cleanName(w.Name)
		if name == "" {
			continue
		}
		// Deduplicate by name + rounded coordinates
		key := fmt.Sprintf("%s|%.3f,%.3f", name, w.Lat, w.Lon)
		if seen[key] {
			continue
		}
		seen[key] = true

		stops = append(stops, stop{
			Lat:  math.Round(w.Lat*10000) / 10000,
			Lng:  math.Round(w.Lon*10000) / 10000,
			Cat:  cat,
			Name: name,
			Desc: strings.TrimSpace(w.Desc),
		})
	}

	// Count by category for reporting
	catCounts := make(map[string]int)
	for _, s := range stops {
		catCounts[s.Cat]++
	}
	fmt.Printf("Waypoints by category:\n")
	for _, cat := range []string{"campsite", "lodging", "bike_shop", "library", "services", "food", "caution"} {
		fmt.Printf("  %-12s %d\n", cat+":", catCounts[cat])
	}
	fmt.Printf("  %-12s %d\n", "total:", len(stops))

	// ── Extract town waypoints (geocache category) ────────────────────────────
	seenTowns := make(map[string]bool)
	var towns []town

	for _, w := range rawWpts {
		if w.Cmt != "geocache" {
			continue
		}
		name := cleanName(w.Name)
		if name == "" {
			continue
		}
		key := fmt.Sprintf("%s|%.3f,%.3f", name, w.Lat, w.Lon)
		if seenTowns[key] {
			continue
		}
		seenTowns[key] = true

		towns = append(towns, town{
			Lat:  math.Round(w.Lat*10000) / 10000,
			Lng:  math.Round(w.Lon*10000) / 10000,
			Name: name,
		})
	}
	fmt.Printf("Towns: %d\n", len(towns))

	stopsJSON, err := json.MarshalIndent(stops, "  ", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshalling stops: %v\n", err)
		os.Exit(1)
	}

	townsJSON, err := json.MarshalIndent(towns, "  ", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshalling towns: %v\n", err)
		os.Exit(1)
	}

	stopsOutput := fmt.Sprintf(
		"// TransAmerica Trail waypoints — generated by tools/process-gpx/main.go\n"+
			"// Source: Adventure Cycling Association GPX files\n"+
			"// DO NOT EDIT — regenerate with: make gpx\n"+
			"\n"+
			"const ROUTE_STOPS = %s;\n"+
			"\n"+
			"const ROUTE_TOWNS = %s;\n",
		"[\n  "+strings.TrimSpace(string(stopsJSON[1:len(stopsJSON)-1]))+"\n]",
		"[\n  "+strings.TrimSpace(string(townsJSON[1:len(townsJSON)-1]))+"\n]",
	)

	if err := os.WriteFile(stopsFile, []byte(stopsOutput), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing stops file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s (%d KB)\n", stopsFile, len(stopsOutput)/1024)
}
