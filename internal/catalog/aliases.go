package catalog

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

//go:embed aliases.csv
var aliasesCSV string

// Row is one curated catalog entry: a canonical movement name in one
// equipment variant, optionally tied to a free-exercise-db id for metadata and images.
type Row struct {
	Name      string
	Equipment string // lowercase token, empty for movements without equipment
	Default   bool   // the variant that the bare name resolves to
	SourceID  string
	Muscles   []string // primary muscles of a stub; a dataset row's own win
	Secondary []string
}

// CuratedRows parses the embedded aliases.csv.
func CuratedRows() ([]Row, error) {
	recs, err := csv.NewReader(strings.NewReader(aliasesCSV)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("catalog: aliases.csv: %w", err)
	}
	rows := make([]Row, 0, len(recs))
	for i, rec := range recs[1:] {
		def, err := strconv.ParseBool(rec[2])
		if err != nil {
			return nil, fmt.Errorf("catalog: aliases.csv row %d: %w", i+2, err)
		}
		rows = append(rows, Row{Name: rec[0], Equipment: rec[1], Default: def, SourceID: rec[3],
			Muscles: muscleList(rec[4]), Secondary: muscleList(rec[5])})
	}
	return rows, nil
}

func muscleList(s string) []string {
	if s == "" {
		return nil
	}
	return lowerAll(strings.Split(s, ";"))
}
