package programs

import (
	"fmt"
	"slices"

	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Structure summarises a program for display: its weeks and the days each
// week has, in week then day order. Foods are the catalog foods a meal
// script gives by name or ref alone, keyed that way, as they were when the
// script was saved.
type Structure struct {
	Name  string
	Weeks int
	Loop  bool
	Days  []StructureDay
	Foods map[string]model.Food `json:",omitempty"`
}

// StructureDay is a day of a program week. Day is 0 for a meal day picked by kind.
type StructureDay struct {
	Week, Day  int
	Name, Kind string
}

// script is an evaluated program: workout or meal, by the program's kind,
// the catalog foods a meal program was evaluated with, and the state block
// of an exported workout script, which only import accepts.
type script struct {
	workout *model.Program
	meal    *model.MealProgram
	foods   map[string]model.Food
	state   *engine.State
}

// evaluate evaluates src as a program of kind; foods, keyed by ref or name,
// fill the foods a meal script gives by ref or name alone.
func evaluate(kind, src string, foods map[string]model.Food) (script, error) {
	var sc script
	var err error
	switch kind {
	case KindWorkout:
		sc.workout, sc.state, err = engine.EvaluateState(src, scriptFile)
	case KindMeal:
		sc.meal, err = engine.EvaluateMealWith(src, scriptFile, foods)
		sc.foods = foods
	default:
		err = invalidArgument(fmt.Errorf("kind must be %s or %s, got %q", KindWorkout, KindMeal, kind))
	}
	return sc, err
}

func (sc script) structure() *Structure {
	if p := sc.workout; p != nil {
		st := &Structure{Name: p.Name, Weeks: p.Weeks, Loop: p.Loop}
		nums := sc.dayNums()
		for week := 1; week <= p.Weeks; week++ {
			for _, n := range nums {
				if d := p.DayAt(week, n); d != nil {
					st.Days = append(st.Days, StructureDay{Week: week, Day: n, Name: d.Name, Kind: d.Kind})
				}
			}
		}
		return st
	}
	m := sc.meal
	st := &Structure{Name: m.Name, Weeks: m.Weeks, Loop: m.Loop, Foods: sc.foods}
	for _, d := range mealDays(m) {
		st.Days = append(st.Days, StructureDay{Week: d.Week, Day: d.Day, Name: d.Meal.Name, Kind: d.Kind})
	}
	return st
}

func (sc script) weeks() int {
	if sc.workout != nil {
		return sc.workout.Weeks
	}
	return sc.meal.Weeks
}

// dayNums are the day numbers the script declares, ascending; 0 is a meal kind day.
func (sc script) dayNums() []int {
	var nums []int
	if sc.workout != nil {
		for _, d := range sc.workout.Days {
			nums = append(nums, d.Num)
		}
	} else {
		for _, d := range sc.meal.Days {
			nums = append(nums, d.Num)
		}
	}
	slices.Sort(nums)
	return slices.Compact(nums)
}

// checkAnchored rejects day numbers that are not weekdays.
func (sc script) checkAnchored() error {
	if n := sc.dayNums(); len(n) > 0 && n[len(n)-1] > 7 {
		return invalidArgument(fmt.Errorf("day %d: a program with a start date numbers days 1 (Monday) to 7 (Sunday)", n[len(n)-1]))
	}
	return nil
}
