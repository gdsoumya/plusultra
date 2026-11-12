# Reference syntax: Bench Press / 3x8 80kg / superset: A
#              Bent Over Row / 3x8 60kg / superset: A
#              Bicep Curl / 3x12 12.5kg / superset: B
#              Triceps Pushdown / 3x12 25kg / superset: B
program("Supersets", weeks=2)
day(1, 1, "Upper", weeks="1-2")
ex("Bench Press", superset="A", warmup=[], sets=[s(3, 8, kg(80))])
ex("Bent Over Row", superset="A", warmup=[], sets=[s(3, 8, kg(60))])
ex("Bicep Curl", eq="dumbbell", superset="B", warmup=[], sets=[s(3, 12, kg(12.5))])
ex("Triceps Pushdown", superset="B", warmup=[], sets=[s(3, 12, kg(25))])
