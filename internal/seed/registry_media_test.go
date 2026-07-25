package seed

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/qrcode"

	"github.com/0funct0ry/squad/internal/db"
)

var pngSig = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func decodePNGFromImage(t *testing.T, b []byte, reader gozxing.Reader) string {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatalf("binary bitmap: %v", err)
	}
	result, err := reader.Decode(bitmap, nil)
	if err != nil {
		t.Fatalf("barcode decode: %v", err)
	}
	return result.GetText()
}

func TestQRCode_MagicBytesAndRoundTrip(t *testing.T) {
	v, err := Generate("qrCode", "BLOB", map[string]any{"content": "hello-squad"})
	if err != nil {
		t.Fatal(err)
	}
	b, ok := v.([]byte)
	if !ok || len(b) == 0 {
		t.Fatalf("expected non-empty []byte, got %T", v)
	}
	if !bytes.HasPrefix(b, pngSig) {
		t.Fatalf("expected PNG signature")
	}
	got := decodePNGFromImage(t, b, qrcode.NewQRCodeReader())
	if got != "hello-squad" {
		t.Errorf("round-trip mismatch: got %q", got)
	}
}

func TestQRCode_SizeClampAndCeiling(t *testing.T) {
	v, err := Generate("qrCode", "BLOB", map[string]any{"size": 2048}) // above clamp of 1024
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(v.([]byte)))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() > 1024 {
		t.Errorf("expected size clamped to <=1024, got %d", img.Bounds().Dx())
	}
	if len(v.([]byte)) > blobSizeCeiling {
		t.Errorf("qrCode exceeded ceiling: %d", len(v.([]byte)))
	}
}

func TestBarcode_AllFormatsRoundTrip(t *testing.T) {
	cases := []struct {
		format  string
		content string
		size    int // width passed to the barcode generator, tuned so the
		// pure-Go gozxing test decoder (which is markedly less tolerant of
		// large 1D-barcode pixel widths than production scanners/zbar/zxing
		// are) can reliably read the result back.
		reader gozxing.Reader
	}{
		{"code128", "ABC123456789", 300, oned.NewCode128Reader()},
		{"ean13", "4006381333931", 250, oned.NewEAN13Reader()},
		{"ean8", "40170725", 250, oned.NewEAN8Reader()},
	}
	for _, c := range cases {
		t.Run(c.format, func(t *testing.T) {
			v, err := Generate("barcode", "BLOB", map[string]any{"format": c.format, "content": c.content, "size": c.size})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			b := v.([]byte)
			if !bytes.HasPrefix(b, pngSig) {
				t.Fatalf("expected PNG signature")
			}
			got := decodePNGFromImage(t, b, c.reader)
			if got != c.content {
				t.Errorf("round-trip mismatch: want %q got %q", c.content, got)
			}
		})
	}
}

func TestBarcode_DefaultContentPerFormat(t *testing.T) {
	for _, format := range []string{"code128", "ean13", "ean8"} {
		v, err := Generate("barcode", "BLOB", map[string]any{"format": format})
		if err != nil {
			t.Fatalf("format %s: %v", format, err)
		}
		if len(v.([]byte)) == 0 {
			t.Errorf("format %s: expected non-empty output", format)
		}
	}
}

func TestBarcode_EAN13InvalidLengthErrors(t *testing.T) {
	_, err := Generate("barcode", "BLOB", map[string]any{"format": "ean13", "content": "12345"})
	if err == nil {
		t.Fatal("expected error for non-13-digit ean13 content")
	}
}

func TestBarcode_EAN8InvalidContentErrors(t *testing.T) {
	_, err := Generate("barcode", "BLOB", map[string]any{"format": "ean8", "content": "notdigits"})
	if err == nil {
		t.Fatal("expected error for non-numeric ean8 content")
	}
}

func TestBarcode_SizeCeiling(t *testing.T) {
	v, err := Generate("barcode", "BLOB", map[string]any{"format": "code128", "size": 800})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.([]byte)) > blobSizeCeiling {
		t.Errorf("barcode exceeded ceiling: %d", len(v.([]byte)))
	}
}

func TestProfilePicture_DeterministicAndDistinctRegions(t *testing.T) {
	opts := map[string]any{"seed": "fixed-seed-42", "size": 128}
	v1, err := Generate("profilePicture", "BLOB", opts)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := Generate("profilePicture", "BLOB", opts)
	if err != nil {
		t.Fatal(err)
	}
	b1, b2 := v1.([]byte), v2.([]byte)
	if !bytes.Equal(b1, b2) {
		t.Fatalf("expected byte-identical PNGs for same seed")
	}
	if !bytes.HasPrefix(b1, pngSig) {
		t.Fatalf("expected PNG signature")
	}

	img, err := png.Decode(bytes.NewReader(b1))
	if err != nil {
		t.Fatal(err)
	}
	corner := img.At(1, 1)
	center := img.At(64, 64)        // head area
	eyeArea := img.At(64-20, 64-16) // approx left eye
	cr, cg, cb, _ := corner.RGBA()
	hr, hg, hb, _ := center.RGBA()
	if cr == hr && cg == hg && cb == hb {
		t.Errorf("expected corner (background) and center (head) colors to differ")
	}
	_ = eyeArea
}

func TestProfilePicture_DifferentSeedsDiffer(t *testing.T) {
	v1, _ := Generate("profilePicture", "BLOB", map[string]any{"seed": "seed-a"})
	v2, _ := Generate("profilePicture", "BLOB", map[string]any{"seed": "seed-b"})
	if bytes.Equal(v1.([]byte), v2.([]byte)) {
		t.Errorf("expected different seeds to (very likely) produce different images")
	}
}

func TestProfilePicture_SizeCeiling(t *testing.T) {
	v, err := Generate("profilePicture", "BLOB", map[string]any{"size": 512})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.([]byte)) > blobSizeCeiling {
		t.Errorf("profilePicture exceeded ceiling: %d", len(v.([]byte)))
	}
}

func TestSVGImage_PrefixAndShapes(t *testing.T) {
	for _, shape := range []string{"circles", "rects", "blob"} {
		v, err := Generate("svgImage", "BLOB", map[string]any{"shape": shape})
		if err != nil {
			t.Fatalf("shape %s: %v", shape, err)
		}
		b := v.([]byte)
		if !bytes.HasPrefix(b, []byte("<svg")) {
			t.Errorf("shape %s: expected <svg prefix, got %q", shape, string(b[:20]))
		}
		if len(b) > blobSizeCeiling {
			t.Errorf("shape %s exceeded ceiling", shape)
		}
	}
	if !typeMatchesAffinity([]byte("x"), "BLOB") {
		t.Fatal("sanity")
	}
}

func TestIcon_PrefixAndNameOption(t *testing.T) {
	v, err := Generate("icon", "BLOB", map[string]any{"name": "star"})
	if err != nil {
		t.Fatal(err)
	}
	b := v.([]byte)
	if !bytes.HasPrefix(b, []byte("<svg")) {
		t.Fatalf("expected <svg prefix")
	}
	if !strings.Contains(string(b), "polygon") {
		t.Errorf("expected star icon content")
	}
}

func TestIcon_ColorReplace(t *testing.T) {
	v, err := Generate("icon", "BLOB", map[string]any{"name": "heart", "color": "#ff00ff"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(v.([]byte)), "#ff00ff") {
		t.Errorf("expected recolored stroke")
	}
}

func TestIcon_UnknownNameErrors(t *testing.T) {
	_, err := Generate("icon", "BLOB", map[string]any{"name": "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown icon name")
	}
}

func TestIcon_RandomDefaultAndCeiling(t *testing.T) {
	for i := 0; i < 5; i++ {
		v, err := Generate("icon", "BLOB", map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if len(v.([]byte)) > blobSizeCeiling {
			t.Errorf("icon exceeded ceiling")
		}
	}
}

// --- soundData -------------------------------------------------------

func parseWAV(t *testing.T, b []byte) (sampleRate int, samples []int16) {
	t.Helper()
	if len(b) < 44 {
		t.Fatalf("wav too short: %d bytes", len(b))
	}
	if string(b[0:4]) != "RIFF" {
		t.Fatalf("missing RIFF marker")
	}
	if string(b[8:12]) != "WAVE" {
		t.Fatalf("missing WAVE marker")
	}
	chunkSize := binary.LittleEndian.Uint32(b[4:8])
	if int(chunkSize)+8 != len(b) {
		t.Fatalf("inconsistent RIFF chunk size: header says %d, actual %d", chunkSize+8, len(b))
	}
	if string(b[12:16]) != "fmt " {
		t.Fatalf("missing fmt chunk")
	}
	sampleRate = int(binary.LittleEndian.Uint32(b[24:28]))
	dataSize := binary.LittleEndian.Uint32(b[40:44])
	if int(dataSize) != len(b)-44 {
		t.Fatalf("inconsistent data chunk size: header says %d, actual %d", dataSize, len(b)-44)
	}
	n := int(dataSize) / 2
	samples = make([]int16, n)
	for i := 0; i < n; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(b[44+i*2 : 46+i*2]))
	}
	return sampleRate, samples
}

// goertzelEnergy computes the single-frequency energy of samples at freq Hz
// using the Goertzel algorithm, a lightweight stand-in for a full FFT.
func goertzelEnergy(samples []int16, sampleRate int, freq float64) float64 {
	n := len(samples)
	if n == 0 {
		return 0
	}
	k := int(0.5 + float64(n)*freq/float64(sampleRate))
	w := 2 * math.Pi * float64(k) / float64(n)
	cw := math.Cos(w)
	coeff := 2 * cw
	var s0, s1, s2 float64
	for _, sm := range samples {
		s0 = float64(sm) + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}
	power := s1*s1 + s2*s2 - coeff*s1*s2
	return power / float64(n)
}

func TestSoundData_AllWaveformsProduceValidWAV(t *testing.T) {
	waveforms := []string{
		"sineTone", "squareWave", "triangleWave", "sawtoothWave",
		"whiteNoise", "pinkNoise", "chirp", "dtmf", "notificationChime", "drumHit",
	}
	for _, wf := range waveforms {
		t.Run(wf, func(t *testing.T) {
			v, err := Generate("soundData", "BLOB", map[string]any{"waveform": wf})
			if err != nil {
				t.Fatal(err)
			}
			b := v.([]byte)
			if len(b) > audioSizeCeiling {
				t.Errorf("%s exceeded ceiling: %d", wf, len(b))
			}
			parseWAV(t, b)
		})
	}
}

func TestSoundData_SineToneFrequencyMatch(t *testing.T) {
	v, err := Generate("soundData", "BLOB", map[string]any{"waveform": "sineTone", "frequency": 1000.0, "durationMs": 500})
	if err != nil {
		t.Fatal(err)
	}
	_, samples := parseWAV(t, v.([]byte))
	target := goertzelEnergy(samples, wavSampleRate, 1000)
	other1 := goertzelEnergy(samples, wavSampleRate, 300)
	other2 := goertzelEnergy(samples, wavSampleRate, 2500)
	if target < other1*5 || target < other2*5 {
		t.Errorf("expected 1000Hz energy to dominate: target=%v other1=%v other2=%v", target, other1, other2)
	}
}

func TestSoundData_ChirpFrequencyRises(t *testing.T) {
	v, err := Generate("soundData", "BLOB", map[string]any{"waveform": "chirp", "startFrequency": 200.0, "endFrequency": 3000.0, "durationMs": 1000})
	if err != nil {
		t.Fatal(err)
	}
	_, samples := parseWAV(t, v.([]byte))
	n := len(samples)
	firstSeg := samples[:n/4]
	lastSeg := samples[3*n/4:]
	lowEnergyFirst := goertzelEnergy(firstSeg, wavSampleRate, 300)
	highEnergyFirst := goertzelEnergy(firstSeg, wavSampleRate, 2800)
	lowEnergyLast := goertzelEnergy(lastSeg, wavSampleRate, 300)
	highEnergyLast := goertzelEnergy(lastSeg, wavSampleRate, 2800)
	if !(lowEnergyFirst > highEnergyFirst) {
		t.Errorf("expected low-freq energy to dominate early in chirp")
	}
	if !(highEnergyLast > lowEnergyLast) {
		t.Errorf("expected high-freq energy to dominate late in chirp")
	}
}

func TestSoundData_DTMFDualPeaks(t *testing.T) {
	v, err := Generate("soundData", "BLOB", map[string]any{"waveform": "dtmf", "digit": "5"})
	if err != nil {
		t.Fatal(err)
	}
	_, samples := parseWAV(t, v.([]byte))
	rowEnergy := goertzelEnergy(samples, wavSampleRate, 770)
	colEnergy := goertzelEnergy(samples, wavSampleRate, 1336)
	offEnergy := goertzelEnergy(samples, wavSampleRate, 2000)
	if rowEnergy < offEnergy*3 || colEnergy < offEnergy*3 {
		t.Errorf("expected DTMF row/col peaks to dominate: row=%v col=%v off=%v", rowEnergy, colEnergy, offEnergy)
	}
}

func TestSoundData_DTMFInvalidDigitErrors(t *testing.T) {
	_, err := Generate("soundData", "BLOB", map[string]any{"waveform": "dtmf", "digit": "Z"})
	if err == nil {
		t.Fatal("expected error for invalid dtmf digit")
	}
}

func TestSoundData_WhiteVsPinkSpectralDifference(t *testing.T) {
	vw, err := Generate("soundData", "BLOB", map[string]any{"waveform": "whiteNoise", "durationMs": 1000})
	if err != nil {
		t.Fatal(err)
	}
	vp, err := Generate("soundData", "BLOB", map[string]any{"waveform": "pinkNoise", "durationMs": 1000})
	if err != nil {
		t.Fatal(err)
	}
	_, white := parseWAV(t, vw.([]byte))
	_, pink := parseWAV(t, vp.([]byte))

	lowW := goertzelEnergy(white, wavSampleRate, 100)
	highW := goertzelEnergy(white, wavSampleRate, 3500)
	lowP := goertzelEnergy(pink, wavSampleRate, 100)
	highP := goertzelEnergy(pink, wavSampleRate, 3500)

	ratioWhite := lowW / (highW + 1)
	ratioPink := lowP / (highP + 1)
	if !(ratioPink > ratioWhite) {
		t.Errorf("expected pink noise to have relatively more low-frequency energy: ratioWhite=%v ratioPink=%v", ratioWhite, ratioPink)
	}
}

func TestSoundData_DrumHitDecays(t *testing.T) {
	v, err := Generate("soundData", "BLOB", map[string]any{"waveform": "drumHit", "decayMs": 200})
	if err != nil {
		t.Fatal(err)
	}
	_, samples := parseWAV(t, v.([]byte))
	n := len(samples)
	rms := func(s []int16) float64 {
		var sum float64
		for _, v := range s {
			sum += float64(v) * float64(v)
		}
		return math.Sqrt(sum / float64(len(s)))
	}
	startRMS := rms(samples[:n/8])
	endRMS := rms(samples[n-n/8:])
	if !(startRMS > endRMS*2) {
		t.Errorf("expected drumHit amplitude to decay: start=%v end=%v", startRMS, endRMS)
	}
}

func TestSoundData_DurationClampAndCeiling(t *testing.T) {
	v, err := Generate("soundData", "BLOB", map[string]any{"waveform": "sineTone", "durationMs": 999999})
	if err != nil {
		t.Fatal(err)
	}
	b := v.([]byte)
	if len(b) > audioSizeCeiling {
		t.Errorf("soundData exceeded ceiling: %d", len(b))
	}
	// clamped to 5000ms max -> 40000 samples -> 80044 bytes
	if len(b) > 44+40000*2+8 {
		t.Errorf("expected durationMs clamp to ~5000ms, got %d bytes", len(b))
	}
}

// --- nameHeuristic -----------------------------------------------------

func TestNameHeuristic_MediaGenerators(t *testing.T) {
	cases := []struct {
		colName  string
		expected string
	}{
		{"avatar", "profilePicture"},
		{"qr_code", "qrCode"},
		{"barcode_value", "barcode"},
		{"icon_col", "icon"},
		{"logo_svg", "svgImage"},
		{"audio_clip", "soundData"},
		{"payload_data", "bytes"}, // falls through to typeFallback
	}
	for _, c := range cases {
		t.Run(c.colName, func(t *testing.T) {
			col := db.ColumnInfo{Name: c.colName, Type: "BLOB"}
			gen, _ := nameHeuristic(col)
			if gen == "" {
				gen, _ = typeFallback(col)
			}
			if gen != c.expected {
				t.Errorf("column %q: expected generator %q, got %q", c.colName, c.expected, gen)
			}
		})
	}
}
