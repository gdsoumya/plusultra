package export

import (
	"cmp"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// ExportProgram returns userID's program id as its script, byte for byte,
// followed after a blank line by its generated block: the state of a
// workout program or the catalog snapshot of a meal program. It also returns
// a file name for it.
func (s *Service) ExportProgram(ctx context.Context, userID, id uuid.UUID) (text, filename string, err error) {
	p, err := s.Programs.Get(ctx, userID, id)
	if err != nil {
		return "", "", err
	}
	text, err = exportProgram(ctx, s.Q, p)
	return text, slug(p.Name) + ".star", err
}

func exportProgram(ctx context.Context, q *sqlc.Queries, p sqlc.Program) (string, error) {
	var block string
	if p.Kind == programs.KindWorkout {
		o, err := programs.LoadOverlay(ctx, q, p.ID)
		if err != nil {
			return "", err
		}
		if block, err = engine.FormatState(o); err != nil {
			return "", err
		}
	} else {
		var st programs.Structure
		if err := json.Unmarshal(p.Structure, &st); err != nil {
			return "", err
		}
		if len(st.Foods) > 0 {
			block = engine.FormatSnapshot(st.Foods)
		}
	}
	if block == "" {
		return p.Script, nil
	}
	script := p.Script
	if !strings.HasSuffix(script, "\n") {
		script += "\n"
	}
	return script + "\n" + block, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug is name as a file name: lower case words joined by dashes.
func slug(name string) string {
	return cmp.Or(strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-"), "program")
}
