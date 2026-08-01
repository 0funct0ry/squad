package udf

import (
	"database/sql/driver"
	"fmt"
	"math"
	"sort"

	"modernc.org/sqlite"
)

const catNumeric = "Numeric / math"

// floatAggregate is a shared AggregateFunction base that collects the
// numeric argument stream (Step/WindowInverse), supporting both plain
// GROUP BY aggregation and window-function use.
type floatAggregate struct {
	vals   []float64
	finish func([]float64) (driver.Value, error)
}

func (a *floatAggregate) Step(ctx *sqlite.FunctionContext, rowArgs []driver.Value) error {
	f, err := argFloat(rowArgs[0])
	if err != nil {
		return err
	}
	a.vals = append(a.vals, f)
	return nil
}

func (a *floatAggregate) WindowInverse(ctx *sqlite.FunctionContext, rowArgs []driver.Value) error {
	f, err := argFloat(rowArgs[0])
	if err != nil {
		return err
	}
	for i, v := range a.vals {
		if v == f {
			a.vals = append(a.vals[:i], a.vals[i+1:]...)
			break
		}
	}
	return nil
}

func (a *floatAggregate) WindowValue(ctx *sqlite.FunctionContext) (driver.Value, error) {
	return a.finish(a.vals)
}

func (a *floatAggregate) Final(ctx *sqlite.FunctionContext) {}

func median(vals []float64) (driver.Value, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2], nil
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2, nil
}

func variance(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var sq float64
	for _, v := range vals {
		sq += (v - mean) * (v - mean)
	}
	return sq / float64(len(vals))
}

func percentile(vals []float64, p float64) (driver.Value, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	if p <= 0 {
		return sorted[0], nil
	}
	if p >= 1 {
		return sorted[len(sorted)-1], nil
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo], nil
	}
	frac := idx - float64(lo)
	return sorted[lo] + (sorted[hi]-sorted[lo])*frac, nil
}

// percentileAggregate additionally tracks the p argument, since MakeAggregate
// gets no args up front.
type percentileAggregate struct {
	vals []float64
	p    float64
	set  bool
}

func (a *percentileAggregate) Step(ctx *sqlite.FunctionContext, rowArgs []driver.Value) error {
	f, err := argFloat(rowArgs[0])
	if err != nil {
		return err
	}
	if !a.set {
		p, err := argFloat(rowArgs[1])
		if err != nil {
			return err
		}
		a.p = p
		a.set = true
	}
	a.vals = append(a.vals, f)
	return nil
}

func (a *percentileAggregate) WindowInverse(ctx *sqlite.FunctionContext, rowArgs []driver.Value) error {
	f, err := argFloat(rowArgs[0])
	if err != nil {
		return err
	}
	for i, v := range a.vals {
		if v == f {
			a.vals = append(a.vals[:i], a.vals[i+1:]...)
			break
		}
	}
	return nil
}

func (a *percentileAggregate) WindowValue(ctx *sqlite.FunctionContext) (driver.Value, error) {
	return percentile(a.vals, a.p)
}

func (a *percentileAggregate) Final(ctx *sqlite.FunctionContext) {}

// modeAggregate tracks value frequency by string representation, returning
// the most frequent original value (first-seen wins ties).
type modeAggregate struct {
	order  []string
	counts map[string]int
	first  map[string]driver.Value
}

func (a *modeAggregate) Step(ctx *sqlite.FunctionContext, rowArgs []driver.Value) error {
	if a.counts == nil {
		a.counts = map[string]int{}
		a.first = map[string]driver.Value{}
	}
	key := argString(rowArgs[0])
	if _, ok := a.counts[key]; !ok {
		a.order = append(a.order, key)
		a.first[key] = rowArgs[0]
	}
	a.counts[key]++
	return nil
}

func (a *modeAggregate) WindowInverse(ctx *sqlite.FunctionContext, rowArgs []driver.Value) error {
	key := argString(rowArgs[0])
	if a.counts[key] > 0 {
		a.counts[key]--
	}
	return nil
}

func (a *modeAggregate) result() (driver.Value, error) {
	var best string
	bestCount := -1
	for _, key := range a.order {
		if a.counts[key] > bestCount {
			bestCount = a.counts[key]
			best = key
		}
	}
	if bestCount <= 0 {
		return nil, nil
	}
	return a.first[best], nil
}

func (a *modeAggregate) WindowValue(ctx *sqlite.FunctionContext) (driver.Value, error) {
	return a.result()
}
func (a *modeAggregate) Final(ctx *sqlite.FunctionContext) {}

func registerNumeric() error {
	aggregates := []struct {
		name   string
		desc   string
		call   string
		result string
		finish func([]float64) (driver.Value, error)
	}{
		{"MEDIAN", "Median of x across the group.", "SELECT MEDIAN(amount) FROM orders", "(the median amount)", median},
		{"STDDEV", "Standard deviation of x.", "SELECT STDDEV(amount) FROM orders", "(the stddev)", func(v []float64) (driver.Value, error) { return math.Sqrt(variance(v)), nil }},
		{"VARIANCE", "Variance of x.", "SELECT VARIANCE(amount) FROM orders", "(the variance)", func(v []float64) (driver.Value, error) { return variance(v), nil }},
	}
	for _, a := range aggregates {
		a := a
		if err := sqlite.RegisterFunction(a.name, &sqlite.FunctionImpl{
			NArgs:         1,
			Deterministic: true,
			MakeAggregate: func(ctx sqlite.FunctionContext) (sqlite.AggregateFunction, error) {
				return &floatAggregate{finish: a.finish}, nil
			},
		}); err != nil {
			return err
		}
		add(Descriptor{Name: a.name, Category: catNumeric, Signature: fmt.Sprintf("%s(x) -> float", a.name),
			Description: a.desc, ExampleCall: a.call, ExampleResult: a.result, Aggregate: true, Deterministic: true})
	}

	if err := sqlite.RegisterFunction("MODE", &sqlite.FunctionImpl{
		NArgs:         1,
		Deterministic: true,
		MakeAggregate: func(ctx sqlite.FunctionContext) (sqlite.AggregateFunction, error) {
			return &modeAggregate{}, nil
		},
	}); err != nil {
		return err
	}
	add(Descriptor{Name: "MODE", Category: catNumeric, Signature: "MODE(x) -> any",
		Description: "Most frequent value of x across the group.", ExampleCall: "SELECT MODE(status) FROM orders",
		ExampleResult: "(the most frequent status)", Aggregate: true, Deterministic: true})

	if err := sqlite.RegisterFunction("PERCENTILE", &sqlite.FunctionImpl{
		NArgs:         2,
		Deterministic: true,
		MakeAggregate: func(ctx sqlite.FunctionContext) (sqlite.AggregateFunction, error) {
			return &percentileAggregate{}, nil
		},
	}); err != nil {
		return err
	}
	add(Descriptor{Name: "PERCENTILE", Category: catNumeric, Signature: "PERCENTILE(x, p) -> float",
		Description: "The pth percentile (0-1) of x across the group.",
		ExampleCall: "SELECT PERCENTILE(amount, 0.95) FROM orders", ExampleResult: "(p95 order value)",
		Aggregate: true, Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("ROUND_TO", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			num, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			places, err := argInt(args[1])
			if err != nil {
				return nil, err
			}
			mult := math.Pow(10, float64(places))
			return math.Round(num*mult) / mult, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "ROUND_TO", Category: catNumeric, Signature: "ROUND_TO(num, places) -> float",
		Description: "Rounds num to places decimal places.", ExampleCall: `ROUND_TO(3.14159, 2)`,
		ExampleResult: "3.14", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("CLAMP", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			num, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			min, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			max, err := argFloat(args[2])
			if err != nil {
				return nil, err
			}
			if num < min {
				return min, nil
			}
			if num > max {
				return max, nil
			}
			return num, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "CLAMP", Category: catNumeric, Signature: "CLAMP(num, min, max) -> float",
		Description: "Clamps num into [min, max].", ExampleCall: `CLAMP(120, 0, 100)`,
		ExampleResult: "100", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("GCD", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			a, err := argInt(args[0])
			if err != nil {
				return nil, err
			}
			b, err := argInt(args[1])
			if err != nil {
				return nil, err
			}
			return gcd(a, b), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "GCD", Category: catNumeric, Signature: "GCD(a, b) -> int",
		Description: "Greatest common divisor.", ExampleCall: `GCD(12, 18)`, ExampleResult: "6", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("LCM", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			a, err := argInt(args[0])
			if err != nil {
				return nil, err
			}
			b, err := argInt(args[1])
			if err != nil {
				return nil, err
			}
			g := gcd(a, b)
			if g == 0 {
				return int64(0), nil
			}
			return (a / g) * b, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "LCM", Category: catNumeric, Signature: "LCM(a, b) -> int",
		Description: "Least common multiple.", ExampleCall: `LCM(4, 6)`, ExampleResult: "12", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("IS_PRIME", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			n, err := argInt(args[0])
			if err != nil {
				return nil, err
			}
			return boolResult(isPrime(n)), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IS_PRIME", Category: catNumeric, Signature: "IS_PRIME(n) -> bool",
		Description: "1 if n is prime, else 0.", ExampleCall: `IS_PRIME(17)`, ExampleResult: "1", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("SAFE_DIVIDE", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			a, err := argFloat(args[0])
			if err != nil {
				return nil, err
			}
			b, err := argFloat(args[1])
			if err != nil {
				return nil, err
			}
			if b == 0 {
				return nil, nil
			}
			return a / b, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "SAFE_DIVIDE", Category: catNumeric, Signature: "SAFE_DIVIDE(a, b) -> float",
		Description: "Returns a / b, or NULL instead of raising an error when b is 0.",
		ExampleCall: "SELECT SAFE_DIVIDE(revenue, order_count) FROM daily_stats", ExampleResult: "NULL on zero-order days",
		Deterministic: true})

	return nil
}

func gcd(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func isPrime(n int64) bool {
	if n < 2 {
		return false
	}
	if n < 4 {
		return true
	}
	if n%2 == 0 {
		return false
	}
	for i := int64(3); i*i <= n; i += 2 {
		if n%i == 0 {
			return false
		}
	}
	return true
}
