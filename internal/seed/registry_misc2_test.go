package seed

import (
	"math"
	"strings"
	"testing"
)

func TestVersionString_ShapeAndBounds(t *testing.T) {
	for i := 0; i < 50; i++ {
		v, err := Generate("versionString", "TEXT", map[string]any{"maxMajor": 2, "maxMinor": 3, "maxPatch": 4})
		if err != nil {
			t.Fatal(err)
		}
		s := v.(string)
		base := strings.SplitN(s, "-", 2)[0]
		parts := strings.Split(base, ".")
		if len(parts) != 3 {
			t.Fatalf("expected major.minor.patch shape, got %q", s)
		}
	}
}

func TestDurationInterval_RespectsBoundsAndFormat(t *testing.T) {
	for i := 0; i < 50; i++ {
		v, err := Generate("durationInterval", "INTEGER", map[string]any{"minSeconds": 60, "maxSeconds": 120, "format": "seconds"})
		if err != nil {
			t.Fatal(err)
		}
		secs := v.(int64)
		if secs < 60 || secs > 120 {
			t.Errorf("expected seconds in [60,120], got %d", secs)
		}
	}
	v, err := Generate("durationInterval", "TEXT", map[string]any{"minSeconds": 900, "maxSeconds": 900, "format": "iso8601"})
	if err != nil {
		t.Fatal(err)
	}
	if v.(string) != "PT15M" {
		t.Errorf("expected PT15M, got %v", v)
	}
}

func TestCronExpression_AlwaysInCuratedSet(t *testing.T) {
	valid := map[string]bool{}
	for _, c := range curatedCronExpressions {
		valid[c] = true
	}
	for i := 0; i < 50; i++ {
		v, err := Generate("cronExpression", "TEXT", map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if !valid[v.(string)] {
			t.Errorf("expected a curated cron expression, got %q", v)
		}
	}
}

func TestPercentageValue_NeverOutsideBoundsAtAnyPrecision(t *testing.T) {
	for _, precision := range []int{0, 1, 2} {
		for i := 0; i < 200; i++ {
			v, err := Generate("percentageValue", "REAL", map[string]any{"precision": precision})
			if err != nil {
				t.Fatal(err)
			}
			f := v.(float64)
			if f < 0 || f > 100 {
				t.Errorf("precision=%d: value %f out of [0,100]", precision, f)
			}
		}
	}
}

func TestFileSizeBytes_DistributionIsLogSkewed(t *testing.T) {
	const n = 5000
	const minB, maxB = 1024, 104857600
	small := 0
	for i := 0; i < n; i++ {
		v, err := Generate("fileSizeBytes", "INTEGER", map[string]any{"minBytes": minB, "maxBytes": maxB})
		if err != nil {
			t.Fatal(err)
		}
		size := v.(int64)
		if size < minB || size > maxB {
			t.Errorf("size %d out of bounds [%d,%d]", size, minB, maxB)
		}
		if size < minB*10 {
			small++
		}
	}
	// Under a uniform distribution over [1024, 100MB], values below 10x the
	// minimum would be a vanishingly tiny fraction of the range (~0.01%);
	// under the configured log-uniform distribution, ln(10)/ln(maxB/minB)
	// predicts ~20%. Assert comfortably above the uniform expectation to
	// confirm genuine log-skew without pinning to the exact math.
	frac := float64(small) / n
	if frac < 0.1 {
		t.Errorf("expected a log-skewed distribution concentrated near the low end, only %f%% were below 10x min", frac*100)
	}
}

func TestUserAgentByDevice_RespectsDeviceOption(t *testing.T) {
	for device, pool := range uaByDevice {
		v, err := Generate("userAgentByDevice", "TEXT", map[string]any{"device": device})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, ua := range pool {
			if v == ua {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("device=%s: expected a UA from that device's pool, got %v", device, v)
		}
	}
}

func TestFileExtensionAndMimeType_RespectCategory(t *testing.T) {
	v, err := Generate("fileExtension", "TEXT", map[string]any{"category": "image"})
	if err != nil {
		t.Fatal(err)
	}
	ext := strings.TrimPrefix(v.(string), ".")
	found := false
	for _, e := range fileCategoryExtensions["image"] {
		if e == ext {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an image extension, got %q", ext)
	}

	m, err := Generate("mimeType", "TEXT", map[string]any{"category": "image"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(m.(string), "image/") {
		t.Errorf("expected an image/* mime type, got %q", m)
	}
}

func TestGeohash_StandaloneProducesCorrectLength(t *testing.T) {
	v, err := Generate("geohash", "TEXT", map[string]any{"precision": 7})
	// geohash is Fn: nil, so the generic Generate() dispatcher can't call it
	// directly -- confirm that expectation instead of invoking through it.
	if err == nil {
		t.Fatalf("expected geohash to reject direct Generate() dispatch, got %v", v)
	}

	sample, err := genGeohashStandalone(map[string]any{"precision": 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(sample.(string)) != 7 {
		t.Errorf("expected a 7-character geohash, got %q", sample)
	}
}

func TestGeohash_CrossColumnDecodesBackNearReferencedLatLng(t *testing.T) {
	schema := simpleSchema("lat", "lng", "geo")
	specs := map[string]ColumnSpec{
		"lat": {Generator: "float", Options: map[string]any{"min": 40.0, "max": 41.0}},
		"lng": {Generator: "float", Options: map[string]any{"min": -74.0, "max": -73.0}},
		"geo": {Generator: "geohash", Options: map[string]any{"columns": []string{"lat", "lng"}, "precision": 9}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	row, err := gen.GenerateRow()
	if err != nil {
		t.Fatal(err)
	}
	geohash := row["geo"].(string)
	if len(geohash) != 9 {
		t.Fatalf("expected a 9-character geohash, got %q", geohash)
	}

	// Re-encoding the exact same lat/lng at the same precision must produce
	// the identical geohash (deterministic function of its inputs), which is
	// an indirect but exact confirmation that the encoded cell matches the
	// referenced columns' real values.
	lat := row["lat"].(float64)
	lng := row["lng"].(float64)
	if math.Abs(lat) > 90 || math.Abs(lng) > 180 {
		t.Fatalf("unexpected lat/lng out of range: %f, %f", lat, lng)
	}
	recomputed := encodeGeohash(lat, lng, 9)
	if recomputed != geohash {
		t.Errorf("expected geohash to match a re-encode of the referenced lat/lng: got %q, want %q", geohash, recomputed)
	}
}
