package udf

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"

	colorful "github.com/lucasb-eyer/go-colorful"
	"modernc.org/sqlite"
)

const catColor = "Color"

func parseHex(s string) (colorful.Color, error) {
	c, err := colorful.Hex(s)
	if err != nil {
		return colorful.Color{}, fmt.Errorf("invalid hex color %q: %w", s, err)
	}
	return c, nil
}

func registerColor() error {
	if err := sqlite.RegisterDeterministicScalarFunction("HEX_TO_RGB", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			c, err := parseHex(argString(args[0]))
			if err != nil {
				return nil, err
			}
			r, g, b := c.RGB255()
			out, _ := json.Marshal(map[string]int{"r": int(r), "g": int(g), "b": int(b)})
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "HEX_TO_RGB", Category: catColor, Signature: "HEX_TO_RGB(hex) -> json",
		Description: "Converts a hex color to {r,g,b} JSON.", ExampleCall: `HEX_TO_RGB('#22d3ee')`,
		ExampleResult: `{"r":34,"g":211,"b":238}`, Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("RGB_TO_HEX", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			r, err := argInt(args[0])
			if err != nil {
				return nil, err
			}
			g, err := argInt(args[1])
			if err != nil {
				return nil, err
			}
			b, err := argInt(args[2])
			if err != nil {
				return nil, err
			}
			c := colorful.Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255}
			return c.Hex(), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "RGB_TO_HEX", Category: catColor, Signature: "RGB_TO_HEX(r, g, b) -> str",
		Description: "Converts r,g,b (0-255) to a hex color.", ExampleCall: `RGB_TO_HEX(34, 211, 238)`,
		ExampleResult: "#22d3ee", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("HEX_TO_HSL", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			c, err := parseHex(argString(args[0]))
			if err != nil {
				return nil, err
			}
			h, s, l := c.Hsl()
			out, _ := json.Marshal(map[string]int{
				"h": int(math.Round(h)), "s": int(math.Round(s * 100)), "l": int(math.Round(l * 100)),
			})
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "HEX_TO_HSL", Category: catColor, Signature: "HEX_TO_HSL(hex) -> json",
		Description: "Converts a hex color to {h,s,l} JSON.", ExampleCall: `HEX_TO_HSL('#22d3ee')`,
		ExampleResult: `{"h":187,"s":85,"l":53}`, Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("COLOR_MIX", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			c1, err := parseHex(argString(args[0]))
			if err != nil {
				return nil, err
			}
			c2, err := parseHex(argString(args[1]))
			if err != nil {
				return nil, err
			}
			ratio, err := argFloat(args[2])
			if err != nil {
				return nil, err
			}
			return c1.BlendRgb(c2, ratio).Hex(), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "COLOR_MIX", Category: catColor, Signature: "COLOR_MIX(hex1, hex2, ratio) -> str",
		Description: "Mixes hex1 and hex2 by ratio (0-1).", ExampleCall: `COLOR_MIX('#ffffff', '#000000', 0.5)`,
		ExampleResult: "#7f7f7f", Deterministic: true})

	lightenDarken := func(sign float64) func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		return func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			c, err := parseHex(argString(args[0]))
			if err != nil {
				return nil, err
			}
			amount, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			h, s, l := c.Hsl()
			l += sign * amount
			if l < 0 {
				l = 0
			}
			if l > 1 {
				l = 1
			}
			return colorful.Hsl(h, s, l).Hex(), nil
		}
	}
	if err := sqlite.RegisterDeterministicScalarFunction("COLOR_LIGHTEN", 2, lightenDarken(1)); err != nil {
		return err
	}
	add(Descriptor{Name: "COLOR_LIGHTEN", Category: catColor, Signature: "COLOR_LIGHTEN(hex, amount) -> str",
		Description: "Lightens hex by amount (0-1).", ExampleCall: `COLOR_LIGHTEN('#22d3ee', 0.2)`,
		ExampleResult: "(a lighter hex color)", Deterministic: true})
	if err := sqlite.RegisterDeterministicScalarFunction("COLOR_DARKEN", 2, lightenDarken(-1)); err != nil {
		return err
	}
	add(Descriptor{Name: "COLOR_DARKEN", Category: catColor, Signature: "COLOR_DARKEN(hex, amount) -> str",
		Description: "Darkens hex by amount (0-1).", ExampleCall: `COLOR_DARKEN('#22d3ee', 0.2)`,
		ExampleResult: "(a darker hex color)", Deterministic: true})

	relLuminance := func(c colorful.Color) float64 {
		lin := func(v float64) float64 {
			if v <= 0.03928 {
				return v / 12.92
			}
			return math.Pow((v+0.055)/1.055, 2.4)
		}
		r, g, b := lin(c.R), lin(c.G), lin(c.B)
		return 0.2126*r + 0.7152*g + 0.0722*b
	}

	if err := sqlite.RegisterDeterministicScalarFunction("COLOR_IS_LIGHT", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			c, err := parseHex(argString(args[0]))
			if err != nil {
				return nil, err
			}
			return boolResult(relLuminance(c) > 0.5), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "COLOR_IS_LIGHT", Category: catColor, Signature: "COLOR_IS_LIGHT(hex) -> bool",
		Description: "Luminance-based light/dark classification.", ExampleCall: `COLOR_IS_LIGHT('#ffffff')`,
		ExampleResult: "1", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("COLOR_CONTRAST_RATIO", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			c1, err := parseHex(argString(args[0]))
			if err != nil {
				return nil, err
			}
			c2, err := parseHex(argString(args[1]))
			if err != nil {
				return nil, err
			}
			l1, l2 := relLuminance(c1)+0.05, relLuminance(c2)+0.05
			if l1 < l2 {
				l1, l2 = l2, l1
			}
			return l1 / l2, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "COLOR_CONTRAST_RATIO", Category: catColor, Signature: "COLOR_CONTRAST_RATIO(hex1, hex2) -> float",
		Description: "WCAG contrast ratio between two colors.", ExampleCall: `COLOR_CONTRAST_RATIO('#000000', '#ffffff')`,
		ExampleResult: "21.0", Deterministic: true})

	return nil
}
