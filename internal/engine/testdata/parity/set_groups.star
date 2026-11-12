# Reference syntax: Squat / 4x5 (Work) 100kg 180s, 2x5 (Backoff) 85kg 120s
#              Bench Press / 1x5 80kg, 1x3 85kg, 1x1 90kg
program("Set groups and labels", weeks=2)
day(1, 1, "Lower", weeks="1-2")
ex("Squat", warmup=[], sets=[
    s(4, 5, kg(100), rest=180, label="Work"),
    s(2, 5, kg(85), rest=120, label="Backoff"),
])
ex("Bench Press", warmup=[], sets=[s(1, 5, kg(80)), s(1, 3, kg(85)), s(1, 1, kg(90))])
