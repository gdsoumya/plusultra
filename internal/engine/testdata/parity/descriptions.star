# Reference syntax:
#   // Intro week
#   # Week 1
#   // Legs first
#   ## Day 1
#   // Pause two seconds at the bottom
#   Squat / 3x5 100kg
#   // Use straps if grip fails
#
#   // No straps from now on
#   Deadlift / 1x5 140kg / progress: custom() {~ if (descriptionIndex == 1) { descriptionIndex = 2 } ~}
#   // Feet high
#   Leg Press / 3x10 120kg
#   # Week 2
#   ## Day 1                 (inherits the day description)
#   Squat / 3x5 105kg        (inherits its description)
#   //
#   Leg Press / 3x10 120kg   (empty // stops it)
program("Descriptions", weeks=2)
week(1, desc="Intro week")
week(2, desc="Heavier")

def next_desc(ctx):
    if ctx.description_index == 1:
        ctx.description_index = 2

day(1, 1, "Lower", desc="Legs first", weeks="1-2")
ex("Squat", desc="Pause two seconds at the bottom", warmup=[], sets=[s(3, 5, kg(100))])
ex("Deadlift", desc=["Use straps if grip fails", "No straps from now on"], warmup=[],
   sets=[s(1, 5, kg(140))], progress=custom(next_desc))
ex("Leg Press", desc="Feet high", warmup=[], sets=[s(3, 10, kg(120))])

# A redeclared day inherits its description too; desc="" would stop it.
day(2, 1, "Lower")
ex("Squat", warmup=[], sets=[s(3, 5, kg(105))])
ex("Leg Press", desc="", warmup=[], sets=[s(3, 10, kg(120))])
