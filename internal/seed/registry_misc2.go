package seed

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// misc2Generators registers the remaining standalone M6c generators:
// versionString, durationInterval, cronExpression, percentageValue,
// fileSizeBytes, userAgentByDevice, fileExtension, mimeType, and geohash.
// All are ordinary Fn-based generators except geohash, which is Fn: nil so
// it can optionally read real sibling lat/lng columns (see evalGeohash).
func misc2Generators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "versionString", Group: "text", Description: "Semantic-version-shaped string", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "preReleaseRate", Label: "Pre-release rate", Kind: OptKindFloat, Default: 0.1, Min: floatPtr(0), Max: floatPtr(1)},
			{Key: "maxMajor", Label: "Max major", Kind: OptKindInt, Default: 5},
			{Key: "maxMinor", Label: "Max minor", Kind: OptKindInt, Default: 20},
			{Key: "maxPatch", Label: "Max patch", Kind: OptKindInt, Default: 50},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			maxMajor := optInt(opts, "maxMajor", 5)
			maxMinor := optInt(opts, "maxMinor", 20)
			maxPatch := optInt(opts, "maxPatch", 50)
			preReleaseRate := optFloat(opts, "preReleaseRate", 0.1)
			v := fmt.Sprintf("%d.%d.%d", gofakeit.Number(0, maxMajor), gofakeit.Number(0, maxMinor), gofakeit.Number(0, maxPatch))
			if rand.Float64() < preReleaseRate {
				kinds := []string{"beta", "rc"}
				v += fmt.Sprintf("-%s.%d", kinds[rand.Intn(len(kinds))], gofakeit.Number(1, 10))
			}
			return v, nil
		}},
		{Name: "durationInterval", Group: "datetime", Description: "A duration value for columns like session_length/sla_window/cook_time", Affinities: []string{"TEXT", "INTEGER"}, OptionsSchema: []OptionField{
			{Key: "format", Label: "Format", Kind: OptKindSelect, Default: "short", Choices: []string{"iso8601", "short", "seconds"}},
			{Key: "minSeconds", Label: "Min seconds", Kind: OptKindInt, Default: 60},
			{Key: "maxSeconds", Label: "Max seconds", Kind: OptKindInt, Default: 14400},
		}, Fn: func(affinity string, opts map[string]any) (any, error) {
			minS := optInt(opts, "minSeconds", 60)
			maxS := optInt(opts, "maxSeconds", 14400)
			format := optString(opts, "format", "short")
			secs := gofakeit.Number(minS, maxS)
			if affinity == "INTEGER" || format == "seconds" {
				return int64(secs), nil
			}
			d := time.Duration(secs) * time.Second
			if format == "iso8601" {
				return isoDuration(d), nil
			}
			return d.String(), nil
		}},
		{Name: "cronExpression", Group: "datetime", Description: "A valid cron string picked from a curated set of common schedules", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return curatedCronExpressions[rand.Intn(len(curatedCronExpressions))], nil
		}},
		{Name: "percentageValue", Group: "numeric", Description: "A bounded [0,100] value", Affinities: []string{"REAL", "INTEGER"}, OptionsSchema: []OptionField{
			{Key: "precision", Label: "Precision", Kind: OptKindInt, Default: 0, Min: floatPtr(0), Max: floatPtr(2)},
		}, Fn: func(affinity string, opts map[string]any) (any, error) {
			precision := optInt(opts, "precision", 0)
			v := rand.Float64() * 100
			mult := math.Pow(10, float64(precision))
			v = math.Round(v*mult) / mult
			if affinity == "INTEGER" {
				return int64(v), nil
			}
			return v, nil
		}},
		{Name: "fileSizeBytes", Group: "numeric", Description: "A plausible, log-distributed file-size integer", Affinities: []string{"INTEGER"}, OptionsSchema: []OptionField{
			{Key: "minBytes", Label: "Min bytes", Kind: OptKindInt, Default: 1024},
			{Key: "maxBytes", Label: "Max bytes", Kind: OptKindInt, Default: 104857600},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			minB := optInt(opts, "minBytes", 1024)
			maxB := optInt(opts, "maxBytes", 104857600)
			if minB < 1 {
				minB = 1
			}
			if maxB < minB {
				maxB = minB
			}
			logMin := math.Log(float64(minB))
			logMax := math.Log(float64(maxB))
			v := math.Exp(logMin + rand.Float64()*(logMax-logMin))
			return int64(v), nil
		}},
		{Name: "userAgentByDevice", Group: "internet", Description: "Like userAgent, but with a device option for a realistic device mix", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "device", Label: "Device", Kind: OptKindSelect, Choices: []string{"mobile", "desktop", "tablet", "bot"}, Description: "Default: random-weighted across all four"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			device := optString(opts, "device", "")
			pool, ok := uaByDevice[device]
			if !ok {
				devices := []string{"mobile", "desktop", "tablet", "bot"}
				pool = uaByDevice[devices[rand.Intn(len(devices))]]
			}
			return pool[rand.Intn(len(pool))], nil
		}},
		{Name: "fileExtension", Group: "identifier", Description: "A file extension for attachment-metadata tables", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "category", Label: "Category", Kind: OptKindSelect, Choices: []string{"document", "image", "archive", "audio", "video", "code"}, Description: "Default: any"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			ext := pickFileExtension(optString(opts, "category", ""))
			return "." + ext, nil
		}},
		{Name: "mimeType", Group: "identifier", Description: "The MIME type matching fileExtension's categories", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "category", Label: "Category", Kind: OptKindSelect, Choices: []string{"document", "image", "archive", "audio", "video", "code"}, Description: "Default: any"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			ext := pickFileExtension(optString(opts, "category", ""))
			if mime, ok := extToMime[ext]; ok {
				return mime, nil
			}
			return "application/octet-stream", nil
		}},
		{Name: "geohash", Group: "geo", Description: "A geohash string, optionally derived from real lat/lng sibling columns", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "precision", Label: "Precision", Kind: OptKindInt, Default: 9},
			{Key: "columns", Label: "Lat/Lng columns", Kind: OptKindColumns, Description: "Optional: exactly 2 columns, latitude then longitude"},
		}, Fn: nil},
	}
}

func isoDuration(d time.Duration) string {
	total := int64(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	var b strings.Builder
	b.WriteString("PT")
	if h > 0 {
		fmt.Fprintf(&b, "%dH", h)
	}
	if m > 0 {
		fmt.Fprintf(&b, "%dM", m)
	}
	if s > 0 || (h == 0 && m == 0) {
		fmt.Fprintf(&b, "%dS", s)
	}
	return b.String()
}

var curatedCronExpressions = []string{
	"* * * * *", "*/5 * * * *", "*/15 * * * *", "0 * * * *", "0 0 * * *",
	"0 0 * * 0", "0 0 1 * *", "30 2 * * *", "0 9-17 * * 1-5", "0 0 1 1 *",
}

var uaByDevice = map[string][]string{
	"mobile": {
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
	},
	"desktop": {
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
	},
	"tablet": {
		"Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14; SM-X910) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	},
	"bot": {
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	},
}

var fileCategoryExtensions = map[string][]string{
	"document": {"pdf", "doc", "docx", "txt", "rtf", "odt"},
	"image":    {"jpg", "png", "gif", "webp", "svg", "bmp"},
	"archive":  {"zip", "tar", "gz", "7z", "rar"},
	"audio":    {"mp3", "wav", "ogg", "flac", "aac"},
	"video":    {"mp4", "mov", "avi", "mkv", "webm"},
	"code":     {"go", "js", "ts", "py", "java", "rb"},
}

var extToMime = map[string]string{
	"pdf": "application/pdf", "doc": "application/msword",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"txt":  "text/plain", "rtf": "application/rtf", "odt": "application/vnd.oasis.opendocument.text",
	"jpg": "image/jpeg", "png": "image/png", "gif": "image/gif", "webp": "image/webp", "svg": "image/svg+xml", "bmp": "image/bmp",
	"zip": "application/zip", "tar": "application/x-tar", "gz": "application/gzip", "7z": "application/x-7z-compressed", "rar": "application/vnd.rar",
	"mp3": "audio/mpeg", "wav": "audio/wav", "ogg": "audio/ogg", "flac": "audio/flac", "aac": "audio/aac",
	"mp4": "video/mp4", "mov": "video/quicktime", "avi": "video/x-msvideo", "mkv": "video/x-matroska", "webm": "video/webm",
	"go": "text/x-go", "js": "text/javascript", "ts": "application/typescript", "py": "text/x-python", "java": "text/x-java-source", "rb": "text/x-ruby",
}

func allExtensions() []string {
	var out []string
	for _, exts := range fileCategoryExtensions {
		out = append(out, exts...)
	}
	return out
}

func pickFileExtension(category string) string {
	exts, ok := fileCategoryExtensions[category]
	if !ok {
		exts = allExtensions()
	}
	return exts[rand.Intn(len(exts))]
}

// ---------------------------------------------------------------------
// geohash
// ---------------------------------------------------------------------

const geohashBase32 = "0123456789bcdefghjkmnpqrstuvwxyz"

func encodeGeohash(lat, lng float64, precision int) string {
	if precision <= 0 {
		precision = 9
	}
	latRange := [2]float64{-90, 90}
	lngRange := [2]float64{-180, 180}
	var geohash strings.Builder
	bit := 0
	ch := 0
	evenBit := true
	for geohash.Len() < precision {
		if evenBit {
			mid := (lngRange[0] + lngRange[1]) / 2
			if lng >= mid {
				ch |= 1 << (4 - bit)
				lngRange[0] = mid
			} else {
				lngRange[1] = mid
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
			geohash.WriteByte(geohashBase32[ch])
			bit = 0
			ch = 0
		}
	}
	return geohash.String()
}

func genGeohashStandalone(opts map[string]any) (any, error) {
	precision := optInt(opts, "precision", 9)
	return encodeGeohash(gofakeit.Latitude(), gofakeit.Longitude(), precision), nil
}

func (g *RowGenerator) evalGeohash(spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	precision := optInt(spec.Options, "precision", 9)
	cols := optStringSlice(spec.Options, "columns")
	if len(cols) == 2 {
		latVal, ok1 := rowSoFar[cols[0]]
		lngVal, ok2 := rowSoFar[cols[1]]
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("geohash: referenced columns not yet generated")
		}
		lat, latOK := toFloat(latVal)
		lng, lngOK := toFloat(lngVal)
		if !latOK || !lngOK {
			return nil, fmt.Errorf("geohash: referenced columns must be numeric")
		}
		return encodeGeohash(lat, lng, precision), nil
	}
	v, err := genGeohashStandalone(spec.Options)
	return v, err
}
