package engine

import (
	"embed"
	"io/fs"
	"slices"
)

// MealGroup is the Group of the meal program builtins.
const MealGroup = "Meal programs"

// BuiltinDoc documents one predeclared function for the LLM reference.
// Example is Starlark; a first line "# fragment" or "# meal fragment" marks
// part of a program, which runs after a program() or meal_program() and day().
type BuiltinDoc struct {
	Group, Name, Signature, Doc, Example string
}

// AttrDoc documents one ctx attribute or method. Read and Write say where it
// can be read and assigned: "progress, update", "progress", "update" or "no".
type AttrDoc struct {
	Name, Type, Read, Write, Doc string
}

// docGroup sets the group of docs.
func docGroup(group string, docs ...BuiltinDoc) []BuiltinDoc {
	for i := range docs {
		docs[i].Group = group
	}
	return docs
}

// Docs lists every builtin by reference section: the workout program's, then the meal program's.
func Docs() []BuiltinDoc {
	return slices.Concat(programDocs, setDocs, valueDocs, progressDocs, funcDocs, mathDocs, mealDocs)
}

// CtxDocs lists every attribute and method of ctx.
func CtxDocs() []AttrDoc { return slices.Clone(ctxDocs) }

//go:embed testdata/examples
var examples embed.FS

// Examples holds the example programs: workout scripts as *.star and meal
// scripts as meals/*.star.
var Examples = func() fs.FS {
	f, err := fs.Sub(examples, "testdata/examples")
	if err != nil {
		panic(err)
	}
	return f
}()
