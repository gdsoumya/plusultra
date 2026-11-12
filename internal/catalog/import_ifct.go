package catalog

import (
	"context"
	_ "embed" // the seed
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

const (
	sourceIFCT      = "ifct"
	kjPerKcal       = 4.184
	compositionsCol = "enerc" // marks the ifct2017 compositions table
)

// IFCT's own energy factors in kJ/g (energies table of the ifct2017 package).
const (
	kjProtein = 17
	kjCarbs   = 17
	kjFat     = 37
	kjFiber   = 8
)

//go:embed data/indian-raw.csv
var ifctSeed string

// langPrefix is the language tag before a local name in IFCT, as in "H. Ananas".
var langPrefix = regexp.MustCompile(`^[A-Za-z]+\.\s*`)

// ImportIFCT loads Indian foods. An empty path loads the embedded seed, which
// mixes IFCT rows (source "ifct", source_id the IFCT code) with reference rows
// for foods IFCT lacks. Otherwise path is compositions/index.csv of the ifct2017
// package, the full table of 542 foods in per-100 g kJ, protein, available
// carbohydrate, fat and fibre. Re-running updates rows by source and source_id.
func ImportIFCT(ctx context.Context, q *sqlc.Queries, path string) (int, error) {
	var src io.Reader = strings.NewReader(ifctSeed)
	if path != "" {
		f, err := os.Open(path) //nolint:gosec // operator-supplied path
		if err != nil {
			return 0, fmt.Errorf("catalog: open dataset: %w", err)
		}
		defer f.Close() //nolint:errcheck // read-only
		src = f
	}
	n := 0
	err := eachRow(src, func(r row) error {
		fr, ok := ifctRow(r)
		if !ok {
			return invalid("ifct row %q has no name, code or energy", r.get("source_id")+r.get("code"))
		}
		if _, err := fr.save(ctx, q, nil); err != nil {
			return fmt.Errorf("catalog: upsert %s: %w", fr.SourceID, err)
		}
		n++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("catalog: read dataset: %w", err)
	}
	return n, nil
}

func ifctRow(r row) (foodRow, bool) {
	if _, ok := r.cols[compositionsCol]; ok {
		kj, ok := parseNum(r.get("enerc"))
		m := model.Macros{Protein: num(r.get("protcnt")), Carbs: num(r.get("choavldf")), Fat: num(r.get("fatce")), Fiber: num(r.get("fibtg"))}
		if kj == 0 { // oils and fats carry no published energy
			kj = kjProtein*m.Protein + kjFat*m.Fat + kjCarbs*m.Carbs + kjFiber*m.Fiber
		}
		m.Kcal = round1(kj / kjPerKcal)
		fr := foodRow{Source: sourceIFCT, SourceID: r.get("code"), Name: r.get("name"), Aliases: localNames(r.get("lang")), Per100g: m}
		return fr, ok && fr.SourceID != "" && fr.Name != ""
	}
	kcal, ok := parseNum(r.get("kcal"))
	fr := foodRow{
		Source: r.get("source"), SourceID: r.get("source_id"), Name: r.get("name"), Aliases: splitList(r.get("aliases")),
		Per100g: model.Macros{
			Kcal: kcal, Protein: num(r.get("protein")), Carbs: num(r.get("carbs")), Fat: num(r.get("fat")), Fiber: num(r.get("fiber")),
		},
	}
	return fr, ok && fr.Source != "" && fr.SourceID != "" && fr.Name != ""
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ";") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// localNames turns "H. Ananas; Tam. Annasi pazham" into the names without language tags.
func localNames(lang string) []string {
	names := splitList(lang)
	for i, n := range names {
		names[i] = langPrefix.ReplaceAllString(n, "")
	}
	return names
}
