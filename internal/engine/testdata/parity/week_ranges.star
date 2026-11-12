# Reference syntax:
#   # Week 1
#   ## Day 1
#   Squat / 3x5 100kg
#   Leg Press[1-2] / 3x10 120kg
#   Leg Curl[1-1] / 3x12 40kg
#   # Week 2
#   ## Day 1
#   Squat / 5x3 110kg
program("Week ranges and overrides", weeks=2)
day(1, 1, "Lower", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))])
ex("Leg Press", weeks="1-2", warmup=[], sets=[s(3, 10, kg(120))])
ex("Leg Curl", weeks=[1], warmup=[], sets=[s(3, 12, kg(40))])

# Week 2 overrides Squat only; Leg Press carries over from the range.
day(2, 1, "Lower heavy")
ex("Squat", warmup=[], sets=[s(5, 3, kg(110))])
