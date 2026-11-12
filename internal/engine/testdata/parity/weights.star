# Reference syntax: Squat / 3x5 75%
#              Bench Press / 3x5 80kg
#              Deadlift / 1x5 225lb      (converted to the user's kg)
program("Percent and absolute weights", weeks=2)
day(1, 1, "Full body", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, pct(75))])
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(80))])
ex("Deadlift", warmup=[], sets=[s(1, 5, lb(225))])
