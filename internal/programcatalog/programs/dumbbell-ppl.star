# Dumbbell PPL, in kg.
#
# Push, pull and legs once a week with dumbbells only; run each day twice a
# week when time allows. Every exercise is three sets in a rep range and adds
# a rep per successful session until all sets reach the top, then goes up
# 2.5 kg and drops back to the bottom. Weights are per dumbbell.
program("Dumbbell PPL", weeks=1, units="kg", notes="Push, pull, legs once a week; run each day twice a week when time allows")

START = "Starting weight is a guess; set your own before the first session by editing the program."

def db(name, lo, hi, weight, rest=90, warmup=None, desc=START):
    ex(name, eq="Dumbbell", desc=desc, warmup=warmup, sets=[s(3, lo, kg(weight), rest=rest)],
       progress=dp(kg(2.5), lo, hi))

def isolation(name, lo, hi, weight):
    db(name, lo, hi, weight, rest=60, warmup=[])

day(1, 1, "Push")
db("Bench Press, Dumbbell", 8, 12, 20)
db("Dumbbell Shoulder Press", 8, 12, 12.5)
db("Incline Bench Press, Dumbbell", 8, 12, 17.5)
isolation("Lateral Raise", 12, 15, 7.5)
isolation("Triceps Extension", 10, 15, 10)

day(1, 3, "Pull")
db("One Arm Row", 8, 12, 20)
db("Pullover", 8, 12, 15, desc=START + " Pull-ups instead when you have a bar: 3x5-8.")
isolation("Reverse Fly", 12, 15, 7.5)
isolation("Bicep Curl", 10, 12, 10)
isolation("Hammer Curl", 10, 12, 10)

day(1, 5, "Legs")
db("Goblet Squat, Dumbbell", 8, 12, 20)
db("Romanian Deadlift, Dumbbell", 8, 12, 20)
db("Lunge", 10, 12, 10, desc=START + " Reps are per leg.")
db("Step Up, Dumbbell", 10, 12, 10, desc=START + " Reps are per leg.")
isolation("Standing Calf Raise, Dumbbell", 15, 20, 20)
