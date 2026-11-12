# Reference syntax: Bench Press / 4x5 80kg, 1x5+ 80kg
#              Pull Up / 3x8-12+ 0kg
program("AMRAP", weeks=2)
day(1, 1, "Upper", weeks="1-2")
ex("Bench Press", warmup=[], sets=[s(4, 5, kg(80)), s(1, 5, kg(80), amrap=True)])
ex("Pull Up", sets=[s(3, (8, 12), bw(), amrap=True)])
