package model

// Macros are nutrition amounts: kcal, grams of each macro, and water in ml.
type Macros struct {
	Kcal, Protein, Carbs, Fat, Fiber float64
	WaterML                          float64
}

// Add returns the field-wise sum of m and o.
func (m Macros) Add(o Macros) Macros {
	return Macros{m.Kcal + o.Kcal, m.Protein + o.Protein, m.Carbs + o.Carbs, m.Fat + o.Fat, m.Fiber + o.Fiber, m.WaterML + o.WaterML}
}

// Scale returns m with every field multiplied by f.
func (m Macros) Scale(f float64) Macros {
	return Macros{m.Kcal * f, m.Protein * f, m.Carbs * f, m.Fat * f, m.Fiber * f, m.WaterML * f}
}

// Food is a food with frozen macros, per 100 g and, for foods counted in
// units such as eggs, per Unit.
type Food struct {
	Name, Ref string
	Per100g   Macros
	PerUnit   *Macros
	Unit      string
	// Line is the script line of the food() call. Unresolved marks a food given by name or ref
	// alone that the script's catalog_snapshot does not fill, to be filled from the catalog.
	// Neither is stored.
	Line       int  `json:"-"`
	Unresolved bool `json:"-"`
}

// Item is a quantity of a food in Unit ("g" or the food's unit) and the macros it gives.
type Item struct {
	Food   Food
	Qty    float64
	Unit   string
	Note   string
	Macros Macros
	// Planned marks an item copied from the meal plan, as opposed to one added ad hoc.
	Planned bool
	// Edited marks an item whose food macros the user overrode; the catalog is not consulted.
	Edited bool
}

// Meal is one meal() declaration.
type Meal struct {
	Name, Time, Desc string
	Items            []Item
}

// MealDay is one day() declaration of a meal program. Num is 0 for a kind
// day, which applies to every workout day of its Kind.
type MealDay struct {
	Week, Num        int
	Kind, Name, Desc string
	Weeks            []int // nil: only Week
	Target           *Macros
	Meals            []Meal
}

// MealProgram is the structure built by evaluating a meal script.
type MealProgram struct {
	Name  string
	Weeks int
	Loop  bool
	Notes string
	Days  []MealDay
}

// Totals sums the macros of every item on d.
func (d *MealDay) Totals() Macros {
	var t Macros
	for _, m := range d.Meals {
		for _, it := range m.Items {
			t = t.Add(it.Macros)
		}
	}
	return t
}

// DayFor returns the meal day for workout day num of week with the given kind:
// the numbered day num active in week, else the kind day active in week or,
// failing that, in the nearest earlier week. A conditioning day without a
// conditioning meal day eats like a training day. Among days active in the
// same week, the one declared for the later week wins, then the later in
// source. It returns nil when nothing matches. The result points into m.Days.
func (m *MealProgram) DayFor(week, num int, kind string) *MealDay {
	if num > 0 {
		if d, at := m.latest(week, func(d *MealDay) bool { return d.Num == num }); at == week {
			return d
		}
	}
	d, _ := m.latest(week, func(d *MealDay) bool { return d.Num == 0 && d.Kind == kind })
	if d == nil && kind == "conditioning" {
		d, _ = m.latest(week, func(d *MealDay) bool { return d.Num == 0 && d.Kind == "training" })
	}
	return d
}

// latest returns the matching day active in the latest week up to week, and that week.
func (m *MealProgram) latest(week int, match func(*MealDay) bool) (*MealDay, int) {
	var best *MealDay
	at := 0
	for i := range m.Days {
		d := &m.Days[i]
		if !match(d) {
			continue
		}
		w := 0
		for _, x := range d.weeks() {
			if x <= week {
				w = max(w, x)
			}
		}
		if w > 0 && (w > at || w == at && d.Week >= best.Week) {
			best, at = d, w
		}
	}
	return best, at
}

func (d *MealDay) weeks() []int {
	if d.Weeks != nil {
		return d.Weeks
	}
	return []int{d.Week}
}
