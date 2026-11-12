package engine

import (
	"maps"
	"slices"
	"testing"
)

// TestDocsMatchBuiltins pins the reference to the engine: every predeclared
// name and every ctx attribute has exactly one doc.
func TestDocsMatchBuiltins(t *testing.T) {
	var program, meal []string
	for _, d := range Docs() {
		if d.Group == MealGroup {
			meal = append(meal, d.Name)
		} else {
			program = append(program, d.Name)
		}
	}
	var ctx []string
	for _, d := range CtxDocs() {
		ctx = append(ctx, d.Name)
	}
	for _, c := range []struct {
		what      string
		docs, has []string
	}{
		{"program builtins", program, slices.Collect(maps.Keys((&builder{}).builtins()))},
		{"meal builtins", meal, slices.Collect(maps.Keys((&mealBuilder{}).builtins()))},
		{"ctx attributes", ctx, (&ctxValue{}).AttrNames()},
	} {
		slices.Sort(c.docs)
		slices.Sort(c.has)
		if !slices.Equal(c.docs, c.has) {
			t.Errorf("%s: documented %v, registered %v", c.what, c.docs, c.has)
		}
	}
	for _, d := range Docs() {
		if d.Group == "" || d.Signature == "" || d.Doc == "" || d.Example == "" {
			t.Errorf("%s: incomplete doc %+v", d.Name, d)
		}
	}
	for _, d := range CtxDocs() {
		if d.Type == "" || d.Read == "" || d.Write == "" || d.Doc == "" {
			t.Errorf("ctx.%s: incomplete doc %+v", d.Name, d)
		}
	}
}
