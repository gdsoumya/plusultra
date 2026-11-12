# Timed, count-up, plain and unilateral sets on every day of the week, for the e2e scripts.
program("Timer check", weeks=2, units="kg", notes="Timed, count-up, rest-only and unilateral sets every day")

def timers(n):
    day(1, n, "Timers day %d" % n, weeks="1-2")
    ex("Bench Press", warmup=[], sets=[s(2, 5, kg(40), rest=30)])
    ex("Jump Rope", sets=[s(3, 1, bw(), work=45, rest=30, auto=True)])
    ex("Plank", sets=[s(2, 1, bw(), work=30, count_up=True, rest=20)])
    ex("Bulgarian Split Squat", eq="Dumbbell", warmup=[], sets=[s(2, 8, kg(12), rest=30)])

def all_days():
    for n in range(1, 8):
        timers(n)

all_days()
