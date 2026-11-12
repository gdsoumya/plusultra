program("P", weeks=1)
day(1, 1, "Mixed")
ex("Squat", sets=[
    s(4, 5, kg(75), rest=180, label="Work"),
    s(3, (8, 12), pct(70)),
    s(1, 5, kg(60), amrap=True),
])
ex("Jump Rope", sets=[s(3, 1, bw(), work=60, rest=30, auto=True)])
ex("Plank", sets=[s(1, 1, bw(), work=30, count_up=True)])
ex("Curl", sets=[s(3, reps_range(10, 15), rpe=8, ask=True, log_rpe=True)])
