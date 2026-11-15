# German Volume Training, after Charles Poliquin, in kg.
#
# Phase 1: three days a week for six weeks, then the program ends and phase 2
# (10 sets of 6 at a heavier weight) takes over. Each day pairs two lifts for
# 10 sets of 10 at one weight, a load you could lift about 20 times, with 90 s
# rest, then two accessories for 3 sets with 60 s rest. A 10x10 lift adds
# weight only once all 100 reps are done; accessories add a rep per successful
# session to the top of their range, then weight.
program("German Volume Training", weeks=6, units="kg", loop=False,
        notes="Phase 1: 10x10 at a slow tempo, four seconds down and two up, three days a week for six weeks")

START = "Starting weight is a guess. Before the first session set a weight you could lift about 20 times by editing the program."
ACCESSORY = "Starting weight is a guess; set your own before the first session by editing the program."

def tens(name, weight, inc, desc=None, eq=None):
    ex(name, eq=eq, desc=desc, superset="A", sets=[s(10, 10, weight, rest=90)], progress=lp(inc))

def accessory(name, lo, hi, weight, eq=None):
    ex(name, eq=eq, desc=ACCESSORY, superset="B", warmup=[], sets=[s(3, lo, weight, rest=60)],
       progress=dp(kg(2.5), lo, hi))

day(1, 1, "Chest and back", weeks="1-6")
tens("Bench Press", pct(60), kg(2.5))
tens("Bent Over Row", kg(50), kg(2.5), desc=START)
accessory("Incline Chest Fly", 10, 12, kg(10), eq="Dumbbell")
accessory("One Arm Row", 10, 12, kg(20), eq="Dumbbell")

day(1, 3, "Legs and abs", weeks="1-6")
tens("Squat", pct(60), kg(5))
tens("Lying Leg Curl", kg(30), kg(2.5), desc=START)
accessory("Standing Calf Raise", 15, 20, kg(60))
ex("Hanging Leg Raise", superset="B", sets=[s(3, (15, 20), bw(), rest=60)])

day(1, 5, "Arms and shoulders", weeks="1-6")
tens("Dip", bw(), kg(2.5), desc="Weight goes on a belt once all 10x10 are done.")
tens("Incline Bicep Curl", kg(7.5), kg(2.5), desc=START, eq="Dumbbell")
accessory("Lateral Raise", 10, 12, kg(7.5), eq="Dumbbell")
accessory("Reverse Fly", 10, 12, kg(7.5), eq="Dumbbell")
