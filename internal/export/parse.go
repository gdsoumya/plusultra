package export

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gdsoumya/plusultra/internal/model"
)

// block is one parsed day of text, with the lines its parts came from.
type block struct {
	Day
	line        int   // the header
	workoutLine int   // the workout line, 0 when none
	mealLines   []int // one per meal
}

var statuses = []string{"planned", "done", "partial", "skipped", "rest"}

// Caps on one import, so an 8 MiB body cannot expand into unbounded memory
// or hours of writes before anything is validated.
const (
	maxSets     = 50_000
	maxBlocks   = 5000
	maxEntries  = 100 // exercises per workout and sets per exercise, what a record holds
	maxSetGroup = 100
	maxReps     = 1000
)

var (
	weightRE  = regexp.MustCompile(`^(\d+(?:\.\d+)?)(kg|lb)$`)
	workoutRE = regexp.MustCompile(`^workout: ("(?:[^"\\]|\\.)*") week (\d+) day (\d+) ("(?:[^"\\]|\\.)*")(?: / (\d+)min)?$`)
	mealsRE   = regexp.MustCompile(`^meals: target (\S+)kcal (\S+)p (\S+)c (\S+)f / actual (\S+)kcal (\S+)p (\S+)c (\S+)f$`)
	groupRE   = regexp.MustCompile(`^(\d+)x(\d+)(?:-(\d+))?(\+)? (\?|\d+(?:\.\d+)?(?:kg|lb))(?: @(\d+(?:\.\d+)?))?(?: \(([^)]*)\))?(?: (✓+)| -> (\S+))?$`)
	resultRE  = regexp.MustCompile(`^(\d+)(?:@(\d+(?:\.\d+)?(?:kg|lb)))?$`)
	itemRE    = regexp.MustCompile(`^(.+) (\d+(?:\.\d+)?)(g| \S+)$`)
)

// parseDays reads day-log text in the export format.
func parseDays(text string) ([]*block, error) {
	if !utf8.ValidString(text) {
		return nil, invalid(1, "text is not valid UTF-8")
	}
	var out []*block
	var cur *block
	section := ""
	sets := 0
	dates := map[string]int{}
	for i, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		n := i + 1
		line = strings.TrimRight(line, "\r")
		switch {
		case line == "":
		case strings.HasPrefix(line, "  "):
			if cur == nil || section == "" {
				return nil, invalid(n, "indented line outside a workout or meals section")
			}
			if section == "workout" {
				if len(cur.Workout.Entries) == maxEntries {
					return nil, invalid(n, "more than %d exercises in one workout", maxEntries)
				}
				e, err := parseEntry(line[2:], n, &sets)
				if err != nil {
					return nil, err
				}
				cur.Workout.Entries = append(cur.Workout.Entries, e)
				continue
			}
			m, err := parseMeal(line[2:], n)
			if err != nil {
				return nil, err
			}
			cur.Meals.Meals, cur.mealLines = append(cur.Meals.Meals, m), append(cur.mealLines, n)
		case strings.HasPrefix(line, "workout: ") && cur != nil:
			if cur.Workout != nil {
				return nil, invalid(n, "a second workout line")
			}
			w, err := parseWorkout(line, n)
			if err != nil {
				return nil, err
			}
			cur.Workout, cur.workoutLine, section = w, n, "workout"
		case strings.HasPrefix(line, "meals: ") && cur != nil:
			if cur.Meals != nil {
				return nil, invalid(n, "a second meals line")
			}
			m, err := parseMealsLine(line, n)
			if err != nil {
				return nil, err
			}
			cur.Meals, section = m, "meals"
		default:
			if len(out) == maxBlocks {
				return nil, invalid(n, "more than %d days in one import", maxBlocks)
			}
			b, err := parseHeader(line, n)
			if err != nil {
				return nil, err
			}
			if first, dup := dates[b.Date]; dup {
				return nil, invalid(n, "%s again; it is already on line %d", b.Date, first)
			}
			dates[b.Date] = n
			out, cur, section = append(out, b), b, ""
		}
	}
	return out, nil
}

func parseHeader(line string, n int) (*block, error) {
	fields := strings.Split(line, " / ")
	if d, err := time.Parse(time.DateOnly, fields[0]); err != nil || d.Format(time.DateOnly) != fields[0] {
		return nil, invalid(n, "expected a date header such as 2026-11-11 / status: done, got %q", fields[0])
	}
	b := &block{Day: Day{Date: fields[0]}, line: n}
	for _, f := range fields[1:] {
		key, val, ok := strings.Cut(f, ": ")
		if !ok {
			return nil, invalid(n, "bad header field %q", f)
		}
		switch key {
		case "status":
			if !slices.Contains(statuses, val) {
				return nil, invalid(n, "status must be one of %s, got %q", strings.Join(statuses, ", "), val)
			}
			b.Status = val
		case "weight":
			w, err := parseWeight(val)
			if err != nil {
				return nil, invalid(n, "bad weight %q", val)
			}
			kg := model.Weight(w).Kg()
			b.Checkin = &Checkin{WeightKg: &kg}
		case "notes":
			s, err := strconv.Unquote(val)
			if err != nil {
				return nil, invalid(n, "notes must be a double-quoted string")
			}
			b.Notes = s
		default:
			return nil, invalid(n, "unknown header field %q", key)
		}
	}
	if b.Status == "" && b.Notes != "" {
		return nil, invalid(n, "notes need a status")
	}
	return b, nil
}

func parseWeight(s string) (model.RecordWeight, error) {
	m := weightRE.FindStringSubmatch(s)
	if m == nil {
		return model.RecordWeight{}, errors.New("bad weight")
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return model.RecordWeight{Value: v, Unit: model.Unit(m[2])}, nil
}

func parseWorkout(line string, n int) (*Workout, error) {
	m := workoutRE.FindStringSubmatch(line)
	if m == nil {
		return nil, invalid(n, `expected workout: "<program>" week <w> day <d> "<day name>"`)
	}
	program, _ := strconv.Unquote(m[1])
	dayName, _ := strconv.Unquote(m[4])
	w := &Workout{Program: program, DayName: dayName}
	w.Week, _ = strconv.Atoi(m[2])
	w.Day, _ = strconv.Atoi(m[3])
	w.Minutes, _ = strconv.Atoi(m[5])
	return w, nil
}

func parseMealsLine(line string, n int) (*Meals, error) {
	m := mealsRE.FindStringSubmatch(line)
	if m == nil {
		return nil, invalid(n, "expected meals: target <kcal>kcal <p>p <c>c <f>f / actual <kcal>kcal <p>p <c>c <f>f")
	}
	var nums [8]float64
	for i := range nums {
		v, err := strconv.ParseFloat(m[i+1], 64)
		if err != nil || v < 0 {
			return nil, invalid(n, "bad number %q", m[i+1])
		}
		nums[i] = v
	}
	return &Meals{Target: model.Macros{Kcal: nums[0], Protein: nums[1], Carbs: nums[2], Fat: nums[3]},
		Totals: model.Macros{Kcal: nums[4], Protein: nums[5], Carbs: nums[6], Fat: nums[7]}}, nil
}

// parseEntry reads an exercise line, counting its sets into total.
func parseEntry(s string, n int, total *int) (Entry, error) {
	fields := strings.Split(s, " / ")
	var e Entry
	e.Name = fields[0]
	if label, name, ok := strings.Cut(fields[0], ": "); ok {
		e.Label, e.Name = label, name
	}
	if e.Name == "" {
		return e, invalid(n, "an exercise line starts with the exercise name")
	}
	for _, f := range fields[1:] {
		if f == "warmup done" {
			e.WarmupDone = true
			continue
		}
		sets, err := parseGroup(f, n, total)
		if err != nil {
			return e, err
		}
		if e.Sets = append(e.Sets, sets...); len(e.Sets) > maxEntries {
			return e, invalid(n, "more than %d sets for one exercise", maxEntries)
		}
	}
	return e, nil
}

// parseGroup reads one set group with its results into the sets it stands
// for. The sets count towards total before they are allocated.
func parseGroup(f string, n int, total *int) ([]model.RecordSet, error) {
	m := groupRE.FindStringSubmatch(f)
	if m == nil {
		return nil, invalid(n, "bad set group %q; expected <sets>x<reps> <weight>", f)
	}
	count, _ := strconv.Atoi(m[1])
	if count < 1 || count > maxSetGroup {
		return nil, invalid(n, "%d sets in one group", count)
	}
	if *total += count; *total > maxSets {
		return nil, invalid(n, "more than %d sets in one import", maxSets)
	}
	p := model.RecordPlanned{Amrap: m[4] == "+", Label: m[7]}
	p.Reps, _ = strconv.Atoi(m[2])
	p.MinReps = p.Reps
	if m[3] != "" {
		p.MinReps, p.Reps = p.Reps, 0
		p.Reps, _ = strconv.Atoi(m[3])
	}
	if m[5] == "?" {
		p.Ask = true
	} else {
		p.Weight, _ = parseWeight(m[5])
	}
	p.RPE, _ = strconv.ParseFloat(m[6], 64)
	if p.Reps > maxReps || p.MinReps > p.Reps {
		return nil, invalid(n, "reps must be at most %d, the range bottom first", maxReps)
	}
	sets := make([]model.RecordSet, count)
	for i := range sets {
		sets[i].Planned = p
	}
	switch {
	case m[8] != "":
		if utf8.RuneCountInString(m[8]) != count {
			return nil, invalid(n, "%q has %d ticks for %d sets", f, utf8.RuneCountInString(m[8]), count)
		}
		for i := range sets {
			sets[i].Completed, sets[i].Reps, sets[i].Weight = true, p.Reps, p.Weight
		}
	case m[9] != "":
		results := strings.Split(m[9], ",")
		if len(results) != count {
			return nil, invalid(n, "%q has %d results for %d sets", f, len(results), count)
		}
		for i, r := range results {
			if r == "-" {
				continue
			}
			rm := resultRE.FindStringSubmatch(r)
			if rm == nil {
				return nil, invalid(n, "bad set result %q; expected <reps>, <reps>@<weight> or -", r)
			}
			sets[i].Completed, sets[i].Weight = true, p.Weight
			if sets[i].Reps, _ = strconv.Atoi(rm[1]); sets[i].Reps > maxReps {
				return nil, invalid(n, "reps must be at most %d", maxReps)
			}
			if rm[2] != "" {
				sets[i].Weight, _ = parseWeight(rm[2])
			} else if asked(p) {
				return nil, invalid(n, "result %q of a ? set needs its weight", r)
			}
		}
	}
	return sets, nil
}

func parseMeal(s string, n int) (model.Meal, error) {
	fields := strings.Split(s, " / ")
	name, adHoc := strings.CutSuffix(fields[0], " (ad hoc)")
	m := model.Meal{Name: name}
	if name == "" {
		return m, invalid(n, "a meal line starts with the meal name")
	}
	for _, f := range fields[1:] {
		im := itemRE.FindStringSubmatch(f)
		if im == nil {
			return m, invalid(n, "bad item %q; expected <food> <qty>g or <food> <qty> <unit>", f)
		}
		qty, _ := strconv.ParseFloat(im[2], 64)
		unit := "g"
		if im[3] != "g" {
			unit = im[3][1:]
		}
		m.Items = append(m.Items, model.Item{Food: model.Food{Name: im[1]}, Qty: qty, Unit: unit, Planned: !adHoc})
	}
	return m, nil
}
