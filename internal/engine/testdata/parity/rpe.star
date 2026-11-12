# Reference syntax: Bench Press / 3x8 @8          (weight from the RPE chart and 1RM)
#              Squat / 3x5 100kg @8
#              Deadlift / 1x5 @8+ 150kg      (log RPE)
#              Leg Press / 3x10 @9           (no 1RM: the lifter enters the weight)
program("RPE", weeks=2)
day(1, 1, "Full body", weeks="1-2")
# rpe= without a weight loads rpe_multiplier(reps, rpe) of the 1RM when it is known.
ex("Bench Press", warmup=[], sets=[s(3, 8, rpe=8)])
ex("Squat", warmup=[], sets=[s(3, 5, kg(100), rpe=8)])
ex("Deadlift", warmup=[], sets=[s(1, 5, kg(150), rpe=8, log_rpe=True)])
ex("Leg Press", sets=[s(3, 10, rpe=9)])
