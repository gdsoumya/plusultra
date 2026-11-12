package catalog

import (
	"encoding/csv"
	"errors"
	"io"
	"strings"
)

// row is one CSV record addressed by header name.
type row struct {
	cols map[string]int
	vals []string
}

func (r row) get(name string) string {
	if i, ok := r.cols[name]; ok && i < len(r.vals) {
		return strings.TrimSpace(r.vals[i])
	}
	return ""
}

// eachRow streams the records of a headed CSV to fn. Lines starting with # are skipped and
// rows with a different field count are tolerated.
func eachRow(r io.Reader, fn func(row) error) error {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = true
	cr.Comment = '#'
	header, err := cr.Read()
	if err != nil {
		return err
	}
	cols := make(map[string]int, len(header))
	for i, h := range header {
		cols[strings.TrimPrefix(strings.TrimSpace(h), "\xef\xbb\xbf")] = i
	}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := fn(row{cols, rec}); err != nil {
			return err
		}
	}
}
