package cli

import (
	"fmt"
	"strings"

	"github.com/0funct0ry/squad/internal/udf"
)

// cmdFunctions implements ".functions", ".functions CATEGORY",
// ".functions NAME", and ".functions NAME --try ARG...". Called from
// dispatchDotCommand's free-text prefix phase (like .mount) because --try's
// arguments legitimately contain spaces/quotes that the generic
// whitespace-tokenized dispatch would shred.
//
// All variants work identically regardless of --write: discovery and
// try-execution never require write mode.
func (s *State) cmdFunctions(rest string) {
	tokens, err := tokenizeMountArgs(rest)
	if err != nil {
		s.shellError(err)
		return
	}

	if len(tokens) == 0 {
		for _, cat := range udf.Catalog() {
			fmt.Fprintf(s.Out, "%-40s (%d)\n", cat.Name, len(cat.Functions))
		}
		return
	}

	name := tokens[0]

	if len(tokens) >= 2 && tokens[1] == "--try" {
		s.cmdFunctionsTry(name, tokens[2:])
		return
	}

	if d, ok := udf.Find(name); ok {
		s.printFunctionDetail(d)
		return
	}

	lowerName := strings.ToLower(name)
	found := false
	for _, cat := range udf.Catalog() {
		if strings.Contains(strings.ToLower(cat.Name), lowerName) {
			found = true
			fmt.Fprintf(s.Out, "%s (%d)\n", cat.Name, len(cat.Functions))
			for _, fn := range cat.Functions {
				fmt.Fprintf(s.Out, "  %-45s %s\n", fn.Signature, fn.Description)
			}
		}
	}
	if !found {
		s.shellError(fmt.Errorf("no function or category matching %q; run .functions to list categories", name))
	}
}

func (s *State) printFunctionDetail(d udf.Descriptor) {
	fmt.Fprintf(s.Out, "%s\n", d.Signature)
	fmt.Fprintf(s.Out, "  category:      %s\n", d.Category)
	fmt.Fprintf(s.Out, "  description:   %s\n", d.Description)
	fmt.Fprintf(s.Out, "  example:       %s -> %s\n", d.ExampleCall, d.ExampleResult)
	fmt.Fprintf(s.Out, "  aggregate:     %v\n", d.Aggregate)
	fmt.Fprintf(s.Out, "  deterministic: %v\n", d.Deterministic)
}

func (s *State) cmdFunctionsTry(name string, tryArgs []string) {
	d, ok := udf.Find(name)
	if !ok {
		s.shellError(fmt.Errorf("unknown function %q; run .functions to list available functions", name))
		return
	}
	if d.Aggregate {
		s.shellError(fmt.Errorf("%s is an aggregate function and can't be tried against bare args", d.Name))
		return
	}

	placeholders := make([]string, len(tryArgs))
	args := make([]any, len(tryArgs))
	for i, a := range tryArgs {
		placeholders[i] = "?"
		args[i] = a
	}
	query := fmt.Sprintf("SELECT %s(%s)", d.Name, strings.Join(placeholders, ", "))

	var result any
	if err := s.DB.QueryRow(query, args...).Scan(&result); err != nil {
		s.shellError(err)
		return
	}
	fmt.Fprintf(s.Out, "%v\n", result)
}

// functionNames returns every registered function's name, for .functions
// completion.
func functionNames() []string {
	var names []string
	for _, cat := range udf.Catalog() {
		for _, fn := range cat.Functions {
			names = append(names, fn.Name)
		}
	}
	return names
}
