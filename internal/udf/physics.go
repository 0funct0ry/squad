package udf

import (
	"database/sql/driver"
	"fmt"
	"math"

	units "github.com/bcicen/go-units"
	"modernc.org/sqlite"
)

const catPhysics = "Physics and Math"

func registerPhysics() error {
	if err := sqlite.RegisterDeterministicScalarFunction("UNIT_CONVERT", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			value, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			from, err := units.Find(argString(args[1]))
			if err != nil {
				return nil, fmt.Errorf("UNIT_CONVERT: unknown unit %q", argString(args[1]))
			}
			to, err := units.Find(argString(args[2]))
			if err != nil {
				return nil, fmt.Errorf("UNIT_CONVERT: unknown unit %q", argString(args[2]))
			}
			v, err := units.ConvertFloat(value, from, to)
			if err != nil {
				return nil, fmt.Errorf("UNIT_CONVERT: %w", err)
			}
			return v.Float(), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "UNIT_CONVERT", Category: catPhysics, Signature: "UNIT_CONVERT(value, from_unit, to_unit) -> float",
		Description: "Converts value between compatible units (length, mass, temperature, energy, ...).",
		ExampleCall: `UNIT_CONVERT(10, 'km', 'mi')`, ExampleResult: "6.21",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("TERMINAL_VELOCITY", 4,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			mass, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			dragCoef, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			area, err := argFloat(args[2])
			if err != nil {
				return nil, err
			}
			density, err := argFloat(args[3])
			if err != nil {
				return nil, err
			}
			const g = 9.80665
			denom := dragCoef * density * area
			if denom <= 0 {
				return nil, fmt.Errorf("TERMINAL_VELOCITY: drag_coef, area, and density must be positive")
			}
			return math.Sqrt((2 * mass * g) / denom), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "TERMINAL_VELOCITY", Category: catPhysics, Signature: "TERMINAL_VELOCITY(mass, drag_coef, area, density) -> float",
		Description: "Terminal velocity (m/s) of a falling object given mass (kg), drag coefficient, cross-sectional area (m^2), and fluid density (kg/m^3).",
		ExampleCall: `TERMINAL_VELOCITY(80, 1.0, 0.7, 1.225)`, ExampleResult: "(an m/s value)",
		Deterministic: true})

	return nil
}
