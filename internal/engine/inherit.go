package engine

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// progressNone is the Kind of progress="none" while a program is built.
const progressNone = "none"

// inheritProgress settles each declaration's progression and update hook once
// the script has run. The first declaration of an exercise (name and label)
// that carries one, by week, then day, then source order, gives it to every
// later declaration without one. "none" turns it off for its own declaration
// only, and state= on an inheriting declaration adds to the inherited state.
// Two different progressions or hooks for one exercise are an error.
func inheritProgress(p *model.Program, file string) error {
	type decl struct {
		week, day int
		e         *model.Exercise
	}
	var ds []decl
	for i := range p.Days {
		d := &p.Days[i]
		for j := range d.Exercises {
			ds = append(ds, decl{slices.Min(d.WeeksOf(&d.Exercises[j])), d.Num, &d.Exercises[j]})
		}
	}
	slices.SortStableFunc(ds, func(a, b decl) int { return cmp.Or(cmp.Compare(a.week, b.week), cmp.Compare(a.day, b.day)) })
	progs := map[model.ProgKey]*model.Progression{}
	hooks := map[model.ProgKey]*model.Hook{}
	for _, x := range ds {
		e := x.e
		k := model.ProgKey{Exercise: e.Names[0], Label: e.Label}
		errAt := func(format string, args ...any) error {
			return &EvalError{File: file, Line: e.Line, Msg: fmt.Sprintf(format, args...)}
		}
		switch pr := e.Progress; {
		case pr == nil:
			e.Progress = progs[k]
		case pr.Kind == progressNone:
			e.Progress = nil
		case pr.Kind == "": // only state=: it adds to the inherited progression
			base := progs[k]
			if base == nil {
				return errAt("ex: state needs progress")
			}
			merged := *base
			merged.State = maps.Clone(base.State)
			maps.Copy(merged.State, pr.State)
			e.Progress = &merged
		case progs[k] == nil:
			progs[k] = pr
		case !sameProgression(progs[k], pr):
			return errAt("progression for %s declared more than once; declare it once (or use a label)", programName(k))
		}
		switch h := e.Update; {
		case h == nil:
			e.Update = hooks[k]
		case h.Fn == nil:
			e.Update = nil
		case hooks[k] == nil:
			hooks[k] = h
		case hooks[k].Fn != h.Fn:
			return errAt("update for %s declared more than once; declare it once (or use a label)", programName(k))
		}
	}
	return nil
}

// sameProgression reports whether a and b progress the same way from the same state.
func sameProgression(a, b *model.Progression) bool {
	if a.Kind != b.Kind || a.Inc != b.Inc || a.IncPct != b.IncPct || a.After != b.After || a.FailAfter != b.FailAfter ||
		a.FailDec != b.FailDec || a.FailReset != b.FailReset || a.Lo != b.Lo || a.Hi != b.Hi || a.Target != b.Target ||
		a.Fn != b.Fn || len(a.State) != len(b.State) {
		return false
	}
	for k, v := range a.State {
		w, ok := b.State[k]
		if !ok || v.Prompt != w.Prompt {
			return false
		}
		if eq, err := starlark.Equal(v.Value, w.Value); err != nil || !eq {
			return false
		}
	}
	return true
}

// programName is how a program line names exercise k: "label: Name", or "Name".
func programName(k model.ProgKey) string {
	if k.Label == "" {
		return k.Exercise
	}
	return k.Label + ": " + k.Exercise
}
