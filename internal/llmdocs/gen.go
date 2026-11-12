package llmdocs

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/gdsoumya/plusultra/internal/engine"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{"group": group}).ParseFS(templateFS, "templates/*.tmpl"))

// data is what the templates render.
type data struct {
	URL             string
	Ctx             []engine.AttrDoc
	Workouts, Meals []example
}

// example is an example script, titled by its first comment line.
type example struct{ Title, Source string }

// render runs a template. Templates and data are built in, and the tests
// render them all, so an error is a bug.
func render(name string, d data) string {
	var b strings.Builder
	if err := templates.ExecuteTemplate(&b, name, d); err != nil {
		panic(err)
	}
	return b.String()
}

// group lists the builtin docs of a reference section.
func group(name string) ([]engine.BuiltinDoc, error) {
	var out []engine.BuiltinDoc
	for _, d := range engine.Docs() {
		if d.Group == name {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no builtins in group %q", name)
	}
	return out, nil
}

func examples(pattern string) []example {
	files, err := fs.Glob(engine.Examples, pattern)
	if err != nil || len(files) == 0 {
		panic(fmt.Sprintf("no examples match %s: %v", pattern, err))
	}
	out := make([]example, len(files))
	for i, f := range files {
		src, err := fs.ReadFile(engine.Examples, f)
		if err != nil {
			panic(err)
		}
		first, _, _ := strings.Cut(string(src), "\n")
		out[i] = example{Title: strings.TrimSuffix(strings.TrimPrefix(first, "# "), "."), Source: string(src)}
	}
	return out
}
