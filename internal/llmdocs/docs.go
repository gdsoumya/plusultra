// Package llmdocs builds the documents that teach an LLM the program
// languages and the day log format: an llms.txt index and four references.
package llmdocs

import (
	"strings"

	"github.com/gdsoumya/plusultra/internal/engine"
)

// Docs are the generated documents, in Markdown.
type Docs struct {
	LLMSTxt, ProgramLanguage, ProgramExamples, MealLanguage, LogFormat string
}

// Build renders the documents, linking each other under publicURL, such as
// "https://lift.example.com".
func Build(publicURL string) Docs {
	d := data{
		URL:      strings.TrimRight(publicURL, "/"),
		Ctx:      engine.CtxDocs(),
		Workouts: examples("*.star"),
		Meals:    examples("meals/*.star"),
	}
	return Docs{
		LLMSTxt:         render("llms.txt.tmpl", d),
		ProgramLanguage: render("program-language.md.tmpl", d),
		ProgramExamples: render("program-examples.md.tmpl", d),
		MealLanguage:    render("meal-language.md.tmpl", d),
		LogFormat:       render("log-format.md.tmpl", d),
	}
}

// Page names under /llms/.
const (
	ProgramLanguage = "program-language.md"
	ProgramExamples = "program-examples.md"
	MealLanguage    = "meal-language.md"
	LogFormat       = "log-format.md"
)

// Page returns the document served under /llms/<name>.
func (d Docs) Page(name string) (string, bool) {
	switch name {
	case ProgramLanguage:
		return d.ProgramLanguage, true
	case ProgramExamples:
		return d.ProgramExamples, true
	case MealLanguage:
		return d.MealLanguage, true
	case LogFormat:
		return d.LogFormat, true
	}
	return "", false
}
