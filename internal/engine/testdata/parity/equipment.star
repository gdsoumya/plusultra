# Reference syntax: Bench Press / 3x8 80kg              (default equipment, barbell)
#              Bench Press, Dumbbell / 3x8 23kg    (rounds to the dumbbells the user has)
#              Lat Pulldown, Cable / 3x10 41kg
# The equipment after the comma picks the user's rounding for it; eq= overrides it.
program("Equipment variants", weeks=2)
day(1, 1, "Upper", weeks="1-2")
ex("Bench Press", eq="barbell", warmup=[], sets=[s(3, 8, kg(80))])
ex("Bench Press, Dumbbell", warmup=[], sets=[s(3, 8, kg(23))])
ex("Lat Pulldown, Cable", eq="cable", warmup=[], sets=[s(3, 10, kg(41))])
