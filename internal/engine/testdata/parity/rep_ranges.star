# Reference syntax: Lat Pulldown / 3x8-12 50kg
#              Bicep Curl / 2x10-15 12.5kg, 1x20 10kg
program("Rep ranges", weeks=2)
day(1, 1, "Upper", weeks="1-2")
ex("Lat Pulldown", warmup=[], sets=[s(3, (8, 12), kg(50))])
ex("Bicep Curl", eq="dumbbell", warmup=[], sets=[s(2, reps_range(10, 15), kg(12.5)), s(1, 20, kg(10))])
