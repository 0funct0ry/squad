package udf

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"modernc.org/sqlite"
)

const catGeo = "Geo (pure math only)"

const earthRadiusKM = 6371.0088
const earthRadiusM = 6371008.8

func toRad(deg float64) float64 { return deg * math.Pi / 180 }
func toDeg(rad float64) float64 { return rad * 180 / math.Pi }

func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKM * c
}

func bearing(lat1, lon1, lat2, lon2 float64) float64 {
	phi1, phi2 := toRad(lat1), toRad(lat2)
	dLon := toRad(lon2 - lon1)
	y := math.Sin(dLon) * math.Cos(phi2)
	x := math.Cos(phi1)*math.Sin(phi2) - math.Sin(phi1)*math.Cos(phi2)*math.Cos(dLon)
	deg := toDeg(math.Atan2(y, x))
	return math.Mod(deg+360, 360)
}

const geohashBase32 = "0123456789bcdefghjkmnpqrstuvwxyz"

func geohashEncode(lat, lon float64, precision int) string {
	latRange := [2]float64{-90, 90}
	lonRange := [2]float64{-180, 180}
	var sb strings.Builder
	bit, ch := 0, 0
	evenBit := true
	for sb.Len() < precision {
		if evenBit {
			mid := (lonRange[0] + lonRange[1]) / 2
			if lon >= mid {
				ch |= 1 << (4 - bit)
				lonRange[0] = mid
			} else {
				lonRange[1] = mid
			}
		} else {
			mid := (latRange[0] + latRange[1]) / 2
			if lat >= mid {
				ch |= 1 << (4 - bit)
				latRange[0] = mid
			} else {
				latRange[1] = mid
			}
		}
		evenBit = !evenBit
		if bit < 4 {
			bit++
		} else {
			sb.WriteByte(geohashBase32[ch])
			bit, ch = 0, 0
		}
	}
	return sb.String()
}

func geohashDecode(hash string) (lat, lon float64, err error) {
	latRange := [2]float64{-90, 90}
	lonRange := [2]float64{-180, 180}
	evenBit := true
	for _, c := range hash {
		idx := strings.IndexRune(geohashBase32, c)
		if idx < 0 {
			return 0, 0, fmt.Errorf("invalid geohash character %q", c)
		}
		for i := 4; i >= 0; i-- {
			bit := (idx >> uint(i)) & 1
			if evenBit {
				mid := (lonRange[0] + lonRange[1]) / 2
				if bit == 1 {
					lonRange[0] = mid
				} else {
					lonRange[1] = mid
				}
			} else {
				mid := (latRange[0] + latRange[1]) / 2
				if bit == 1 {
					latRange[0] = mid
				} else {
					latRange[1] = mid
				}
			}
			evenBit = !evenBit
		}
	}
	return (latRange[0] + latRange[1]) / 2, (lonRange[0] + lonRange[1]) / 2, nil
}

type geojsonPolygon struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

func parsePolygon(s string) (*geojsonPolygon, error) {
	var p geojsonPolygon
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, err
	}
	if len(p.Coordinates) == 0 || len(p.Coordinates[0]) < 3 {
		return nil, fmt.Errorf("polygon must have an exterior ring with at least 3 points")
	}
	return &p, nil
}

// pointInRing uses the standard ray-casting algorithm on [lon, lat] pairs.
func pointInRing(lat, lon float64, ring [][]float64) bool {
	inside := false
	n := len(ring)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		xi, yi := ring[i][0], ring[i][1]
		xj, yj := ring[j][0], ring[j][1]
		if (yi > lat) != (yj > lat) &&
			lon < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

// polygonAreaM2 approximates area via equirectangular projection around the
// ring's mean latitude, then the planar shoelace formula.
func polygonAreaM2(ring [][]float64) float64 {
	var sumLat float64
	for _, p := range ring {
		sumLat += p[1]
	}
	meanLat := toRad(sumLat / float64(len(ring)))
	cosLat := math.Cos(meanLat)

	proj := make([][2]float64, len(ring))
	for i, p := range ring {
		x := toRad(p[0]) * cosLat * earthRadiusM
		y := toRad(p[1]) * earthRadiusM
		proj[i] = [2]float64{x, y}
	}
	var area float64
	n := len(proj)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		area += proj[j][0]*proj[i][1] - proj[i][0]*proj[j][1]
	}
	return math.Abs(area) / 2
}

func polygonCentroid(ring [][]float64) (lat, lon float64) {
	var sumLat, sumLon float64
	for _, p := range ring {
		sumLon += p[0]
		sumLat += p[1]
	}
	n := float64(len(ring))
	return sumLat / n, sumLon / n
}

func registerGeo() error {
	if err := sqlite.RegisterDeterministicScalarFunction("HAVERSINE_DISTANCE", 4,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			lat1, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			lon1, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			lat2, err := argFloat(args[2])
			if err != nil {
				return nil, err
			}
			lon2, err := argFloat(args[3])
			if err != nil {
				return nil, err
			}
			return haversineKM(lat1, lon1, lat2, lon2), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "HAVERSINE_DISTANCE", Category: catGeo, Signature: "HAVERSINE_DISTANCE(lat1, lon1, lat2, lon2) -> float",
		Description: "Great-circle distance in kilometers between two lat/lon points.",
		ExampleCall: `HAVERSINE_DISTANCE(40.7128, -74.0060, 51.5074, -0.1278)`, ExampleResult: "~5570",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("GEOHASH_ENCODE", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			lat, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			lon, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			precision, err := argInt(args[2])
			if err != nil {
				return nil, err
			}
			return geohashEncode(lat, lon, int(precision)), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "GEOHASH_ENCODE", Category: catGeo, Signature: "GEOHASH_ENCODE(lat, lon, precision) -> str",
		Description: "Encodes a lat/lon pair into a geohash string of the given precision.",
		ExampleCall: `GEOHASH_ENCODE(40.7128, -74.0060, 7)`, ExampleResult: "dr5regw",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("GEOHASH_DECODE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			lat, lon, err := geohashDecode(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("GEOHASH_DECODE: %w", err)
			}
			out, _ := json.Marshal(map[string]float64{"lat": lat, "lon": lon})
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "GEOHASH_DECODE", Category: catGeo, Signature: "GEOHASH_DECODE(hash) -> json",
		Description: "Decodes a geohash back into {lat, lon} JSON.",
		ExampleCall: `GEOHASH_DECODE('dr5regw')`, ExampleResult: `{"lat":40.712,"lon":-74.006}`,
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("BOUNDING_BOX_CONTAINS", 6,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			vals := make([]float64, 6)
			for i := range vals {
				v, err := argFloat(args[i])
				if err != nil {
					return nil, err
				}
				vals[i] = v
			}
			lat, lon, minLat, minLon, maxLat, maxLon := vals[0], vals[1], vals[2], vals[3], vals[4], vals[5]
			return boolResult(lat >= minLat && lat <= maxLat && lon >= minLon && lon <= maxLon), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "BOUNDING_BOX_CONTAINS", Category: catGeo, Signature: "BOUNDING_BOX_CONTAINS(lat, lon, minLat, minLon, maxLat, maxLon) -> bool",
		Description: "1 if the point lies within the given box.",
		ExampleCall: `BOUNDING_BOX_CONTAINS(40.7, -74.0, 40.0, -75.0, 41.0, -73.0)`, ExampleResult: "1",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("BEARING", 4,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			lat1, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			lon1, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			lat2, err := argFloat(args[2])
			if err != nil {
				return nil, err
			}
			lon2, err := argFloat(args[3])
			if err != nil {
				return nil, err
			}
			return bearing(lat1, lon1, lat2, lon2), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "BEARING", Category: catGeo, Signature: "BEARING(lat1, lon1, lat2, lon2) -> float",
		Description: "Initial compass bearing (degrees) from point 1 to point 2.",
		ExampleCall: `BEARING(40.7128, -74.0060, 51.5074, -0.1278)`, ExampleResult: "~51.2",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("POINT_IN_POLYGON", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			lat, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			lon, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			poly, err := parsePolygon(argString(args[2]))
			if err != nil {
				return nil, fmt.Errorf("POINT_IN_POLYGON: %w", err)
			}
			return boolResult(pointInRing(lat, lon, poly.Coordinates[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "POINT_IN_POLYGON", Category: catGeo, Signature: "POINT_IN_POLYGON(lat, lon, geojson_polygon) -> bool",
		Description:   "1 if the point falls inside the GeoJSON polygon.",
		ExampleCall:   `POINT_IN_POLYGON(1, 1, '{"type":"Polygon","coordinates":[[[0,0],[0,2],[2,2],[2,0],[0,0]]]}')`,
		ExampleResult: "1", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("POLYGON_AREA", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			poly, err := parsePolygon(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("POLYGON_AREA: %w", err)
			}
			return polygonAreaM2(poly.Coordinates[0]), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "POLYGON_AREA", Category: catGeo, Signature: "POLYGON_AREA(geojson_polygon) -> float",
		Description: "Area of a GeoJSON polygon in square meters.",
		ExampleCall: "SELECT POLYGON_AREA(region_geojson) FROM regions", ExampleResult: "(area in m^2)",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("POLYGON_CENTROID", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			poly, err := parsePolygon(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("POLYGON_CENTROID: %w", err)
			}
			lat, lon := polygonCentroid(poly.Coordinates[0])
			out, _ := json.Marshal(map[string]float64{"lat": lat, "lon": lon})
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "POLYGON_CENTROID", Category: catGeo, Signature: "POLYGON_CENTROID(geojson_polygon) -> json",
		Description: "Centroid of a GeoJSON polygon as {lat, lon} JSON.",
		ExampleCall: "SELECT POLYGON_CENTROID(region_geojson) FROM regions", ExampleResult: `{"lat":...,"lon":...}`,
		Deterministic: true})

	return nil
}
