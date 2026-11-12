package model

import "slices"

type exerciseKey struct{ name, label string }

func (e *Exercise) key() exerciseKey { return exerciseKey{e.Names[0], e.Label} }

func (d *Day) weeks() []int {
	if d.Weeks != nil {
		return d.Weeks
	}
	return []int{d.Week}
}

// WeeksOf returns the weeks e is placed in when declared on d.
func (d *Day) WeeksOf(e *Exercise) []int {
	if e.Weeks != nil {
		return e.Weeks
	}
	return d.weeks()
}

// DayAt returns the effective day num of week, or nil when nothing is declared
// there. For the day header and for each exercise (name and label), a
// declaration under a later week overrides one under an earlier week for the
// weeks it covers; source order breaks ties. The day's and each exercise's
// description take effect in the first week their declaration is active and
// carry forward to declarations that leave them out. The result shares slices
// with p; treat it as read-only.
func (p *Program) DayAt(week, num int) *Day {
	descs := map[exerciseKey][]string{}
	var desc string
	appliedDay, applied := map[*Day]bool{}, map[*Exercise]bool{}
	var day *Day
	var exs []*Exercise
	for w := 1; w <= week; w++ {
		day, exs = p.declared(w, num)
		if day != nil && day.DescSet && !appliedDay[day] {
			desc, appliedDay[day] = day.Desc, true
		}
		for _, e := range exs {
			if e.Desc != nil && !applied[e] {
				descs[e.key()] = e.Desc
				applied[e] = true
			}
		}
	}
	if day == nil {
		return nil
	}
	out := *day
	out.Week, out.Weeks, out.Desc = week, nil, desc
	out.Exercises = make([]Exercise, len(exs))
	for i, e := range exs {
		out.Exercises[i] = *e
		out.Exercises[i].Desc = descs[e.key()]
	}
	return &out
}

// declared returns the winning declaration of day num active in week and the
// winning exercises active there, in first-declared order.
func (p *Program) declared(week, num int) (*Day, []*Exercise) {
	type slot struct{ i, week int }
	var day *Day
	var exs []*Exercise
	slots := map[exerciseKey]slot{}
	for i := range p.Days {
		d := &p.Days[i]
		if d.Num != num {
			continue
		}
		wins := day == nil || d.Week >= day.Week
		if wins && slices.Contains(d.weeks(), week) {
			day = d
		}
		for j := range d.Exercises {
			e := &d.Exercises[j]
			if !slices.Contains(d.WeeksOf(e), week) {
				continue
			}
			if wins {
				day = d
			}
			s, ok := slots[e.key()]
			switch {
			case !ok:
				slots[e.key()] = slot{len(exs), d.Week}
				exs = append(exs, e)
			case d.Week >= s.week:
				slots[e.key()] = slot{s.i, d.Week}
				exs[s.i] = e
			}
		}
	}
	return day, exs
}
