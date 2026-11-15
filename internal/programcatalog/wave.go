package programcatalog

// wave holds the percentage and wave programs built on 5/3/1 and the classic intermediate 5x5 templates.
var wave = []Entry{
	{Slug: "531-boring-but-big", Name: "5/3/1 Boring But Big", Author: "Jim Wendler", Level: "intermediate", Days: 4,
		Summary: "Four days a week over a four-week cycle, one main lift a day. Each day works the 5/3/1 sets with the last one for as many reps as possible, then 5x10 of the same lift at half the training max and one accessory for 5x10. Week 4 is a deload, after which the maxes go up."},
	{Slug: "nsuns-lp", Name: "nSuns 5/3/1 LP", Author: "nSuns", Level: "intermediate", Days: 4,
		Summary: "Four days a week, looping weekly. Each day has a main lift over nine sets that climb to a 1+ single and pyramid back down, a second lift over eight lighter sets, and accessories on double progression. The reps of the 1+ set decide how much the training max rises for the next week."},
	{Slug: "madcow-5x5", Name: "Madcow 5x5", Author: "Bill Starr and Madcow", Level: "intermediate", Days: 3,
		Summary: "Monday ramps five sets of five to a top set in squat, bench and row, Wednesday is light with press and deadlift, and Friday ramps to a triple above Monday's top set and a back-off eight. Making the triple raises the lift 2.5% for the next week."},
	{Slug: "texas-method", Name: "Texas Method", Author: "Mark Rippetoe and Glenn Pendlay", Level: "intermediate", Days: 3,
		Summary: "Monday is volume with squat and a press for 5x5 and a heavy deadlift set, Wednesday is light, and Friday is one set of five for a new best in squat and the press, then power cleans. Bench and press alternate weekly, and Friday's new best sets the next Monday's weights."},
	{Slug: "smolov-jr", Name: "Smolov Jr", Author: "Sergey Smolov", Level: "advanced", Days: 4,
		Summary: "Three weeks of one lift four days a week: 6x6 at 70%, 7x5 at 75%, 8x4 at 80% and 10x3 at 85% of the max. Every set gains 2.5 kg for bench or 5 kg for squat in weeks 2 and 3; retest the max afterwards."},
}
