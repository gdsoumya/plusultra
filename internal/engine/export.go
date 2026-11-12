package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	starlarkjson "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/gdsoumya/plusultra/internal/model"
)

// State is the state({...}) block of an exported workout program: its
// overlay, and the script line the block starts on.
type State struct {
	Overlay model.Overlay
	Line    int
}

// EvaluateState is Evaluate that also returns the script's state({...})
// block, nil when it has none.
func EvaluateState(src, filename string) (*model.Program, *State, error) {
	b := &builder{}
	if _, err := exec(src, filename, b.builtins()); err != nil {
		return nil, nil, err
	}
	if b.prog == nil {
		return nil, nil, &EvalError{File: filename, Msg: "program() was never called"}
	}
	if err := inheritProgress(b.prog, filename); err != nil {
		return nil, nil, err
	}
	return b.prog, b.state, nil
}

// stateBlock accepts the block FormatState writes: the overlay's JSON as a dict.
func (b *builder) stateBlock(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var d *starlark.Dict
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "overlay", &d); err != nil {
		return nil, err
	}
	if b.state != nil {
		return nil, fmt.Errorf("%s: called more than once", fn.Name())
	}
	enc, err := starlark.Call(t, starlarkjson.Module.Members["encode"], starlark.Tuple{d}, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	o, err := UnmarshalOverlay([]byte(enc.(starlark.String)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	b.state = &State{Overlay: o, Line: callerLine(t)}
	return starlark.None, nil
}

// FormatState writes o as the state({...}) block export appends to a script,
// or "" when o holds nothing.
func FormatState(o model.Overlay) (string, error) {
	raw, err := MarshalOverlay(o)
	if err != nil {
		return "", err
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return "", err
	}
	maps.DeleteFunc(fields, func(_ string, v any) bool { m, isMap := v.(map[string]any); return v == nil || isMap && len(m) == 0 })
	if len(fields) == 0 {
		return "", nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	if err := enc.Encode(fields); err != nil {
		return "", err
	}
	return "state(" + starlarkLiteral(bytes.TrimSpace(buf.Bytes())) + ")\n", nil
}

// starlarkLiteral turns JSON into the same value in Starlark syntax: only the
// three keywords differ, and they never occur inside a string.
func starlarkLiteral(j []byte) string {
	var b strings.Builder
	for i := 0; i < len(j); {
		rest := j[i:]
		switch {
		case j[i] == '"':
			end := i + 1
			for end < len(j) && j[end] != '"' {
				if j[end] == '\\' {
					end++
				}
				end++
			}
			b.Write(j[i : end+1])
			i = end + 1
		case bytes.HasPrefix(rest, []byte("null")):
			b.WriteString("None")
			i += 4
		case bytes.HasPrefix(rest, []byte("true")):
			b.WriteString("True")
			i += 4
		case bytes.HasPrefix(rest, []byte("false")):
			b.WriteString("False")
			i += 5
		default:
			b.WriteByte(j[i])
			i++
		}
	}
	return b.String()
}

// FormatSnapshot writes foods, keyed as a meal program's structure keys them,
// as the catalog_snapshot({...}) block export appends to a script.
func FormatSnapshot(foods map[string]model.Food) string {
	var b strings.Builder
	b.WriteString("catalog_snapshot({\n")
	for _, k := range slices.Sorted(maps.Keys(foods)) {
		f := foods[k]
		fmt.Fprintf(&b, "    %s: ", strconv.Quote(k))
		if f.PerUnit == nil {
			b.WriteString(macrosLiteral(f.Per100g))
		} else {
			fmt.Fprintf(&b, "{\"per100g\": %s, \"per_unit\": %s, \"unit\": %s}", macrosLiteral(f.Per100g), macrosLiteral(*f.PerUnit), strconv.Quote(f.Unit))
		}
		b.WriteString(",\n")
	}
	b.WriteString("})\n")
	return b.String()
}

var macroOrder = []string{"kcal", "protein", "carbs", "fat", "fiber", "water_ml"}

// macrosLiteral is m as a macro dict, zero fields left out.
func macrosLiteral(m model.Macros) string {
	var parts []string
	for _, k := range macroOrder {
		if v := *macroFields[k](&m); v != 0 {
			parts = append(parts, fmt.Sprintf("%q: %s", k, strconv.FormatFloat(v, 'g', -1, 64)))
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// SplitExport cuts the state({...}) and catalog_snapshot({...}) blocks export
// appended off the end of src, with the blank line before them, and reports
// whether src is a meal program. Export appends at most one block of each
// name, so at most one trailing statement of each is cut and a script's own
// trailing catalog_snapshot stays. Text that does not parse comes back whole.
func SplitExport(src string) (script string, meal bool) {
	f, err := fileOptions.Parse("export.star", src, 0)
	if err != nil {
		return src, false
	}
	for _, st := range f.Stmts {
		meal = meal || callName(st) == "meal_program"
	}
	cut := -1
	seen := map[string]bool{}
	for i := len(f.Stmts) - 1; i >= 0; i-- {
		n := callName(f.Stmts[i])
		if n != "state" && n != "catalog_snapshot" || seen[n] {
			break
		}
		seen[n] = true
		start, _ := f.Stmts[i].Span()
		cut = int(start.Line)
	}
	if cut < 0 {
		return src, meal
	}
	script = lineStart(src, cut)
	if strings.HasSuffix(script, "\n\n") {
		script = script[:len(script)-1]
	}
	return script, meal
}

// lineStart is src up to the start of 1-based line n.
func lineStart(src string, n int) string {
	off := 0
	for i := 1; i < n; i++ {
		j := strings.IndexByte(src[off:], '\n')
		if j < 0 {
			return src
		}
		off += j + 1
	}
	return src[:off]
}

// callName is the function name a top-level call statement calls, or "".
func callName(st syntax.Stmt) string {
	es, ok := st.(*syntax.ExprStmt)
	if !ok {
		return ""
	}
	call, ok := es.X.(*syntax.CallExpr)
	if !ok {
		return ""
	}
	if id, ok := call.Fn.(*syntax.Ident); ok {
		return id.Name
	}
	return ""
}
