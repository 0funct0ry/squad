package seed

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/ean"
	"github.com/brianvoe/gofakeit/v7"
	qrcode "github.com/skip2/go-qrcode"
)

// Size ceilings enforced by every media generator. Images/SVG/icon/QR/barcode
// generators must stay within blobSizeCeiling; soundData waveforms (which are
// larger by nature, being uncompressed PCM) get audioSizeCeiling.
const (
	blobSizeCeiling  = 64 * 1024
	audioSizeCeiling = 128 * 1024
)

//go:embed data/icons/*.svg
var iconFS embed.FS

var iconNames = func() []string {
	entries, err := iconFS.ReadDir("data/icons")
	if err != nil {
		panic(fmt.Sprintf("seed: failed to read embedded icons: %v", err))
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".svg"))
	}
	sort.Strings(names)
	return names
}()

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func mediaGenerators() []GeneratorDef {
	return []GeneratorDef{
		{
			Name:        "qrCode",
			Group:       "media",
			Description: "QR code image (PNG)",
			Affinities:  []string{"BLOB"},
			OptionsSchema: []OptionField{
				{Key: "content", Label: "Content", Kind: OptKindString, Description: "Text/URL encoded in the QR code (default: random UUID)"},
				{Key: "size", Label: "Size (px)", Kind: OptKindInt, Default: 256, Min: floatPtr(64), Max: floatPtr(1024)},
			},
			Fn: genQRCode,
		},
		{
			Name:        "barcode",
			Group:       "media",
			Description: "Barcode image (PNG)",
			Affinities:  []string{"BLOB"},
			OptionsSchema: []OptionField{
				{Key: "format", Label: "Format", Kind: OptKindSelect, Default: "code128", Choices: []string{"code128", "ean13", "ean8"}},
				{Key: "content", Label: "Content", Kind: OptKindString, Description: "Barcode content (default: random digits)"},
				{Key: "size", Label: "Size (px)", Kind: OptKindInt, Default: 300, Min: floatPtr(100), Max: floatPtr(800)},
			},
			Fn: genBarcode,
		},
		{
			Name:        "profilePicture",
			Group:       "media",
			Description: "Deterministic cartoon avatar (PNG)",
			Affinities:  []string{"BLOB"},
			OptionsSchema: []OptionField{
				{Key: "seed", Label: "Seed", Kind: OptKindString, Description: "Deterministic seed; same seed -> identical image (default: random per row)"},
				{Key: "size", Label: "Size (px)", Kind: OptKindInt, Default: 128, Min: floatPtr(32), Max: floatPtr(512)},
			},
			Fn: genProfilePicture,
		},
		{
			Name:        "svgImage",
			Group:       "media",
			Description: "Hand-built SVG image",
			Affinities:  []string{"BLOB", "TEXT"},
			OptionsSchema: []OptionField{
				{Key: "shape", Label: "Shape", Kind: OptKindSelect, Default: "circles", Choices: []string{"circles", "rects", "blob"}},
				{Key: "size", Label: "Size (px)", Kind: OptKindInt, Default: 200, Min: floatPtr(50), Max: floatPtr(800)},
			},
			Fn: genSVGImage,
		},
		{
			Name:        "icon",
			Group:       "media",
			Description: "Curated icon (SVG), embedded set",
			Affinities:  []string{"BLOB", "TEXT"},
			OptionsSchema: []OptionField{
				{Key: "name", Label: "Icon name", Kind: OptKindSelect, Choices: iconNames, Description: "Default: random from embedded set"},
				{Key: "color", Label: "Color (hex)", Kind: OptKindString, Description: "Recolors stroke/fill, e.g. #ff0000"},
			},
			Fn: genIcon,
		},
		{
			Name:        "soundData",
			Group:       "media",
			Description: "Synthesized WAV audio",
			Affinities:  []string{"BLOB"},
			OptionsSchema: []OptionField{
				{Key: "waveform", Label: "Waveform", Kind: OptKindSelect, Default: "sineTone", Choices: []string{
					"sineTone", "squareWave", "triangleWave", "sawtoothWave", "whiteNoise", "pinkNoise",
					"chirp", "dtmf", "notificationChime", "drumHit",
				}},
				{Key: "durationMs", Label: "Duration (ms)", Kind: OptKindInt, Default: 500, Min: floatPtr(50), Max: floatPtr(5000), Description: "Ignored by dtmf/notificationChime/drumHit"},
				{Key: "frequency", Label: "Frequency (Hz)", Kind: OptKindFloat, Default: 440},
				{Key: "startFrequency", Label: "Start frequency (Hz)", Kind: OptKindFloat, Default: 200},
				{Key: "endFrequency", Label: "End frequency (Hz)", Kind: OptKindFloat, Default: 2000},
				{Key: "digit", Label: "DTMF digit", Kind: OptKindString, Default: "5"},
				{Key: "decayMs", Label: "Decay (ms)", Kind: OptKindInt, Default: 150, Min: floatPtr(30), Max: floatPtr(500)},
			},
			Fn: genSoundData,
		},
	}
}

// ---------------------------------------------------------------------
// qrCode
// ---------------------------------------------------------------------

func genQRCode(_ string, opts map[string]any) (any, error) {
	content := optString(opts, "content", gofakeit.UUID())
	size := clampInt(optInt(opts, "size", 256), 64, 1024)

	png, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("qrCode: %w", err)
	}
	if len(png) > blobSizeCeiling {
		return nil, fmt.Errorf("qrCode: generated image (%d bytes) exceeds %d byte ceiling", len(png), blobSizeCeiling)
	}
	return png, nil
}

// ---------------------------------------------------------------------
// barcode
// ---------------------------------------------------------------------

func genBarcode(_ string, opts map[string]any) (any, error) {
	format := optString(opts, "format", "code128")
	_, userSuppliedContent := opts["content"]
	content := optString(opts, "content", "")
	size := clampInt(optInt(opts, "size", 300), 100, 800)

	var bc barcode.Barcode
	var err error

	switch format {
	case "code128":
		if content == "" {
			content = gofakeit.DigitN(12)
		}
		bc, err = code128.Encode(content)
	case "ean13":
		if userSuppliedContent && content != "" {
			if !isAllDigits(content) || len(content) != 13 {
				return nil, fmt.Errorf("barcode: ean13 content must be exactly 13 digits, got %q", content)
			}
		} else {
			content = gofakeit.DigitN(12)
		}
		bc, err = ean.Encode(content)
	case "ean8":
		if userSuppliedContent && content != "" {
			if !isAllDigits(content) || len(content) != 8 {
				return nil, fmt.Errorf("barcode: ean8 content must be exactly 8 digits, got %q", content)
			}
		} else {
			content = gofakeit.DigitN(7)
		}
		bc, err = ean.Encode(content)
	default:
		return nil, fmt.Errorf("barcode: unsupported format %q (want code128, ean13, or ean8)", format)
	}
	if err != nil {
		return nil, fmt.Errorf("barcode: %w", err)
	}

	height := size / 4
	if height < 40 {
		height = 40
	}
	scaled, err := barcode.Scale(bc, size, height)
	if err != nil {
		return nil, fmt.Errorf("barcode: scale: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, fmt.Errorf("barcode: encode png: %w", err)
	}
	if buf.Len() > blobSizeCeiling {
		return nil, fmt.Errorf("barcode: generated image (%d bytes) exceeds %d byte ceiling", buf.Len(), blobSizeCeiling)
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------
// profilePicture
// ---------------------------------------------------------------------

var skinPalette = []color.RGBA{
	{255, 224, 189, 255},
	{240, 184, 135, 255},
	{198, 134, 66, 255},
	{141, 85, 36, 255},
	{92, 51, 23, 255},
}

var hairPalette = []color.RGBA{
	{20, 20, 20, 255},
	{90, 60, 30, 255},
	{200, 160, 60, 255},
	{230, 230, 230, 255},
	{120, 40, 20, 255},
}

func genProfilePicture(_ string, opts map[string]any) (any, error) {
	seed := optString(opts, "seed", gofakeit.UUID())
	size := clampInt(optInt(opts, "size", 128), 32, 512)

	h := sha256.Sum256([]byte(seed))

	bg := color.RGBA{h[0], h[1], h[2], 255}
	skin := skinPalette[int(h[3])%len(skinPalette)]
	hasHair := h[4]%2 == 0
	hair := hairPalette[int(h[5])%len(hairPalette)]
	smile := h[6]%2 == 0
	eyeOffset := int(h[7]%5) - 2 // -2..2

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	cx, cy := size/2, size/2
	radius := float64(size) * 0.35

	fillCircle := func(cx, cy int, r float64, col color.RGBA, predicate func(x, y int, dist float64) bool) {
		minX := cx - int(r) - 1
		maxX := cx + int(r) + 1
		minY := cy - int(r) - 1
		maxY := cy + int(r) + 1
		for y := minY; y <= maxY; y++ {
			if y < 0 || y >= size {
				continue
			}
			for x := minX; x <= maxX; x++ {
				if x < 0 || x >= size {
					continue
				}
				dx := float64(x - cx)
				dy := float64(y - cy)
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist <= r {
					if predicate == nil || predicate(x, y, dist) {
						img.SetRGBA(x, y, col)
					}
				}
			}
		}
	}

	// Head.
	fillCircle(cx, cy, radius, skin, nil)

	// Optional hair: a cap across the top third of the head.
	if hasHair {
		hairCut := float64(cy) - radius*0.5
		fillCircle(cx, cy, radius, hair, func(_, y int, _ float64) bool {
			return float64(y) < hairCut
		})
	}

	// Eyes.
	eyeR := radius / 8
	if eyeR < 1 {
		eyeR = 1
	}
	eyeXOff := radius / 2.2
	eyeY := cy - int(radius/4) + eyeOffset
	eyeColor := color.RGBA{30, 30, 30, 255}
	fillCircle(cx-int(eyeXOff), eyeY, eyeR, eyeColor, nil)
	fillCircle(cx+int(eyeXOff), eyeY, eyeR, eyeColor, nil)

	// Mouth.
	mouthColor := color.RGBA{120, 40, 40, 255}
	mouthY := cy + int(radius/2)
	mouthHalfW := radius / 2.5
	if smile {
		for dx := -mouthHalfW; dx <= mouthHalfW; dx++ {
			x := cx + int(dx)
			if x < 0 || x >= size {
				continue
			}
			// Downward-curving parabola approximating a smile.
			curve := (dx * dx) / (mouthHalfW * mouthHalfW) * (radius / 6)
			y := mouthY + int(curve) - int(radius/8)
			for t := 0; t < 2; t++ {
				yy := y + t
				if yy >= 0 && yy < size {
					img.SetRGBA(x, yy, mouthColor)
				}
			}
		}
	} else {
		thickness := int(radius/10) + 1
		for dx := -mouthHalfW; dx <= mouthHalfW; dx++ {
			x := cx + int(dx)
			if x < 0 || x >= size {
				continue
			}
			for t := 0; t < thickness; t++ {
				y := mouthY + t
				if y >= 0 && y < size {
					img.SetRGBA(x, y, mouthColor)
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("profilePicture: encode png: %w", err)
	}
	if buf.Len() > blobSizeCeiling {
		return nil, fmt.Errorf("profilePicture: generated image (%d bytes) exceeds %d byte ceiling", buf.Len(), blobSizeCeiling)
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------
// svgImage
// ---------------------------------------------------------------------

func genSVGImage(affinity string, opts map[string]any) (any, error) {
	shape := optString(opts, "shape", "circles")
	size := clampInt(optInt(opts, "size", 200), 50, 800)

	var body strings.Builder
	switch shape {
	case "circles":
		n := 3 + gofakeit.Number(0, 3)
		for i := 0; i < n; i++ {
			r := gofakeit.Number(size/12, size/4)
			cx := gofakeit.Number(r, size-r)
			cy := gofakeit.Number(r, size-r)
			fmt.Fprintf(&body, `<circle cx="%d" cy="%d" r="%d" fill="%s"/>`, cx, cy, r, gofakeit.HexColor())
		}
	case "rects":
		n := 3 + gofakeit.Number(0, 3)
		for i := 0; i < n; i++ {
			w := gofakeit.Number(size/8, size/3)
			h := gofakeit.Number(size/8, size/3)
			x := gofakeit.Number(0, size-w)
			y := gofakeit.Number(0, size-h)
			fmt.Fprintf(&body, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`, x, y, w, h, gofakeit.HexColor())
		}
	case "blob":
		cx, cy := size/2, size/2
		baseR := float64(size) / 3
		points := 8
		var path strings.Builder
		for i := 0; i <= points; i++ {
			angle := 2 * math.Pi * float64(i%points) / float64(points)
			r := baseR * (0.75 + 0.5*gofakeit.Float64Range(0, 1))
			x := float64(cx) + r*math.Cos(angle)
			y := float64(cy) + r*math.Sin(angle)
			if i == 0 {
				fmt.Fprintf(&path, "M%.1f,%.1f ", x, y)
			} else {
				fmt.Fprintf(&path, "L%.1f,%.1f ", x, y)
			}
		}
		path.WriteString("Z")
		fmt.Fprintf(&body, `<path d="%s" fill="%s"/>`, path.String(), gofakeit.HexColor())
	default:
		return nil, fmt.Errorf("svgImage: unsupported shape %q (want circles, rects, or blob)", shape)
	}

	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d"><rect width="100%%" height="100%%" fill="%s"/>%s</svg>`,
		size, size, size, size, gofakeit.HexColor(), body.String(),
	)
	if len(svg) > blobSizeCeiling {
		return nil, fmt.Errorf("svgImage: generated svg (%d bytes) exceeds %d byte ceiling", len(svg), blobSizeCeiling)
	}
	if affinity == "TEXT" {
		return svg, nil
	}
	return []byte(svg), nil
}

// ---------------------------------------------------------------------
// icon
// ---------------------------------------------------------------------

func genIcon(affinity string, opts map[string]any) (any, error) {
	name := optString(opts, "name", "")
	if name == "" {
		name = iconNames[gofakeit.Number(0, len(iconNames)-1)]
	}
	found := false
	for _, n := range iconNames {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("icon: unknown icon name %q", name)
	}

	data, err := iconFS.ReadFile("data/icons/" + name + ".svg")
	if err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	content := string(data)

	if col := optString(opts, "color", ""); col != "" {
		content = strings.ReplaceAll(content, "#000000", col)
	}

	if len(content) > blobSizeCeiling {
		return nil, fmt.Errorf("icon: generated svg (%d bytes) exceeds %d byte ceiling", len(content), blobSizeCeiling)
	}
	if affinity == "TEXT" {
		return content, nil
	}
	return []byte(content), nil
}

// ---------------------------------------------------------------------
// soundData
// ---------------------------------------------------------------------

const wavSampleRate = 8000

var dtmfRows = map[byte]float64{
	'1': 697, '2': 697, '3': 697, 'A': 697,
	'4': 770, '5': 770, '6': 770, 'B': 770,
	'7': 852, '8': 852, '9': 852, 'C': 852,
	'*': 941, '0': 941, '#': 941, 'D': 941,
}

var dtmfCols = map[byte]float64{
	'1': 1209, '4': 1209, '7': 1209, '*': 1209,
	'2': 1336, '5': 1336, '8': 1336, '0': 1336,
	'3': 1477, '6': 1477, '9': 1477, '#': 1477,
	'A': 1633, 'B': 1633, 'C': 1633, 'D': 1633,
}

func genSoundData(_ string, opts map[string]any) (any, error) {
	waveform := optString(opts, "waveform", "sineTone")
	durationMs := clampInt(optInt(opts, "durationMs", 500), 50, 5000)

	var samples []float64

	switch waveform {
	case "sineTone":
		freq := clampFreq(optFloat(opts, "frequency", 440))
		samples = genPeriodic(durationMs, func(t float64) float64 { return math.Sin(2 * math.Pi * freq * t) })
	case "squareWave":
		freq := clampFreq(optFloat(opts, "frequency", 440))
		samples = genPeriodic(durationMs, func(t float64) float64 {
			if math.Sin(2*math.Pi*freq*t) >= 0 {
				return 1
			}
			return -1
		})
	case "triangleWave":
		freq := clampFreq(optFloat(opts, "frequency", 440))
		samples = genPeriodic(durationMs, func(t float64) float64 {
			return 2 / math.Pi * math.Asin(math.Sin(2*math.Pi*freq*t))
		})
	case "sawtoothWave":
		freq := clampFreq(optFloat(opts, "frequency", 440))
		samples = genPeriodic(durationMs, func(t float64) float64 {
			phase := freq*t - math.Floor(freq*t+0.5)
			return 2 * phase
		})
	case "whiteNoise":
		samples = genWhiteNoise(durationMs)
	case "pinkNoise":
		samples = genPinkNoise(durationMs)
	case "chirp":
		startFreq := clampFreq(optFloat(opts, "startFrequency", 200))
		endFreq := clampFreq(optFloat(opts, "endFrequency", 2000))
		samples = genChirp(durationMs, startFreq, endFreq)
	case "dtmf":
		digit := optString(opts, "digit", "5")
		samples = genDTMF(digit)
		if samples == nil {
			return nil, fmt.Errorf("soundData: unsupported dtmf digit %q", digit)
		}
	case "notificationChime":
		samples = genNotificationChime()
	case "drumHit":
		decayMs := clampInt(optInt(opts, "decayMs", 150), 30, 500)
		samples = genDrumHit(decayMs)
	default:
		return nil, fmt.Errorf("soundData: unsupported waveform %q", waveform)
	}

	estBytes := 44 + len(samples)*2
	if estBytes > audioSizeCeiling {
		return nil, fmt.Errorf("soundData: generated audio (%d bytes) exceeds %d byte ceiling", estBytes, audioSizeCeiling)
	}

	wav := encodeWAV(samples)
	if len(wav) > audioSizeCeiling {
		return nil, fmt.Errorf("soundData: generated audio (%d bytes) exceeds %d byte ceiling", len(wav), audioSizeCeiling)
	}
	return wav, nil
}

func clampFreq(f float64) float64 {
	if f < 20 {
		return 20
	}
	if f > 4000 {
		return 4000
	}
	return f
}

func numSamplesFor(durationMs int) int {
	return durationMs * wavSampleRate / 1000
}

func genPeriodic(durationMs int, fn func(t float64) float64) []float64 {
	n := numSamplesFor(durationMs)
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(wavSampleRate)
		out[i] = fn(t)
	}
	return out
}

func genWhiteNoise(durationMs int) []float64 {
	n := numSamplesFor(durationMs)
	out := make([]float64, n)
	rng := rand.New(rand.NewSource(1)) // fixed source is fine; determinism not required
	for i := 0; i < n; i++ {
		out[i] = rng.Float64()*2 - 1
	}
	return out
}

// genPinkNoise implements the Paul Kellet pink-noise approximation: several
// leaky integrators applied to white noise, which concentrates more energy
// in the low frequencies relative to white noise.
func genPinkNoise(durationMs int) []float64 {
	n := numSamplesFor(durationMs)
	out := make([]float64, n)
	rng := rand.New(rand.NewSource(2))
	var b0, b1, b2, b3, b4, b5, b6 float64
	for i := 0; i < n; i++ {
		white := rng.Float64()*2 - 1
		b0 = 0.99886*b0 + white*0.0555179
		b1 = 0.99332*b1 + white*0.0750759
		b2 = 0.96900*b2 + white*0.1538520
		b3 = 0.86650*b3 + white*0.3104856
		b4 = 0.55000*b4 + white*0.5329522
		b5 = -0.7616*b5 - white*0.0168980
		pink := b0 + b1 + b2 + b3 + b4 + b5 + b6 + white*0.5362
		b6 = white * 0.115926
		out[i] = pink * 0.11
	}
	return out
}

func genChirp(durationMs int, startFreq, endFreq float64) []float64 {
	n := numSamplesFor(durationMs)
	out := make([]float64, n)
	durationSec := float64(durationMs) / 1000
	for i := 0; i < n; i++ {
		t := float64(i) / float64(wavSampleRate)
		phase := 2 * math.Pi * (startFreq*t + (endFreq-startFreq)*t*t/(2*durationSec))
		out[i] = math.Sin(phase)
	}
	return out
}

func genDTMF(digit string) []float64 {
	if len(digit) != 1 {
		return nil
	}
	d := strings.ToUpper(digit)[0]
	row, ok1 := dtmfRows[d]
	col, ok2 := dtmfCols[d]
	if !ok1 || !ok2 {
		return nil
	}
	const durationMs = 200
	n := numSamplesFor(durationMs)
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(wavSampleRate)
		out[i] = 0.5*math.Sin(2*math.Pi*row*t) + 0.5*math.Sin(2*math.Pi*col*t)
	}
	return out
}

func genNotificationChime() []float64 {
	notes := []float64{523.25, 659.25, 783.99, 1046.50} // C5 E5 G5 C6
	const noteMs = 150
	noteSamples := numSamplesFor(noteMs)
	out := make([]float64, 0, noteSamples*len(notes))
	for _, freq := range notes {
		for i := 0; i < noteSamples; i++ {
			t := float64(i) / float64(wavSampleRate)
			envT := float64(i) / float64(noteSamples)
			env := envelopeAttackDecay(envT)
			out = append(out, math.Sin(2*math.Pi*freq*t)*env)
		}
	}
	return out
}

// envelopeAttackDecay gives a short linear attack followed by an exponential
// decay across the note's [0,1] normalized time, avoiding clicks at note
// boundaries.
func envelopeAttackDecay(t float64) float64 {
	const attack = 0.08
	if t < attack {
		return t / attack
	}
	decayT := (t - attack) / (1 - attack)
	return math.Exp(-3 * decayT)
}

func genDrumHit(decayMs int) []float64 {
	n := numSamplesFor(decayMs)
	out := make([]float64, n)
	rng := rand.New(rand.NewSource(3))
	tau := float64(decayMs) / 1000 / 5
	for i := 0; i < n; i++ {
		t := float64(i) / float64(wavSampleRate)
		noise := rng.Float64()*2 - 1
		tone := math.Sin(2 * math.Pi * 90 * t)
		env := math.Exp(-t / tau)
		out[i] = (0.6*noise + 0.4*tone) * env
	}
	return out
}

func encodeWAV(samples []float64) []byte {
	dataSize := len(samples) * 2
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16)) // fmt chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // mono
	binary.Write(&buf, binary.LittleEndian, uint32(wavSampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(wavSampleRate*2)) // byte rate
	binary.Write(&buf, binary.LittleEndian, uint16(2))               // block align
	binary.Write(&buf, binary.LittleEndian, uint16(16))              // bits per sample
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))
	for _, s := range samples {
		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		v := int16(s * 32767)
		binary.Write(&buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}
