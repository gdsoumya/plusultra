# 16-Week Strength & Recomp, bulk block: weeks 1-8 of the plan.
#
# Seven days a week. Monday, Thursday and Sunday are easy bodyweight
# conditioning. Tuesday and Friday train the upper body, Wednesday and
# Saturday the lower body.
#
# Main lifts (bench, deadlift, squat) follow a per-week table: volume in
# weeks 1-3, a light week 4, a strength block in weeks 5-7, then a taper with
# test singles in week 8. Accessories run weeks 1-7 with double progression.
# Labels keep the two Lat Pulldown slots apart: heavy on Tuesday, volume on
# Friday.
program("16-Week Strength & Recomp — Bulk block", weeks=8, units="kg", loop=False,
        notes="Bulk block of a 16-week plan: 4 lifting days, 3 conditioning days")

week(4, desc="Light week")
week(5, desc="Strength block")
week(7, desc="Last heavy week")
week(8, desc="Taper and test")

# Warmups ramp to the first work set; pct() here is of that set, not of 1RM.
BAR_WARMUP = [w(1, 10, kg(20)), w(1, 8, pct(50)), w(1, 5, pct(70)), w(1, 3, pct(85)), w(1, 1, pct(93))]
DEADLIFT_WARMUP = [w(1, 8, kg(60)), w(1, 5, pct(60)), w(1, 3, pct(75)), w(1, 2, pct(88)), w(1, 1, pct(95))]

# week: (sets, reps, work kg, backoff kg or None, rest seconds). Backoffs are 2x5 with 120 s rest.
BENCH = {
    1: (4, 5, 75, 62.5, 180),
    2: (4, 5, 77.5, 65, 180),
    3: (4, 5, 80, 67.5, 180),
    4: (3, 3, 72.5, None, 180),
    5: (4, 3, 85, 72.5, 180),
    6: (4, 3, 87.5, 75, 180),
    7: (3, 2, 92.5, None, 240),
}
DEADLIFT = {
    1: (3, 5, 110, 92.5, 180),
    2: (3, 5, 115, 97.5, 180),
    3: (3, 5, 120, 102.5, 180),
    4: (3, 3, 110, None, 180),
    5: (3, 3, 130, 110, 180),
    6: (3, 3, 137.5, 117.5, 180),
    7: (2, 2, 145, None, 240),
}
SQUAT = {
    1: (4, 5, 77.5, 65, 180),
    2: (4, 5, 82.5, 70, 180),
    3: (4, 5, 87.5, 75, 180),
    4: (3, 3, 82.5, None, 180),
    5: (4, 3, 95, 80, 180),
    6: (4, 3, 100, 85, 180),
    7: (3, 2, 107.5, None, 240),
}

def main_lift(name, table, warmup, desc):
    """Places name in each week of table, with that week's work and backoff sets."""
    for week, (n, reps, work, backoff, rest) in table.items():
        sets = [s(n, reps, kg(work), rest=rest, label="Work")]
        if backoff:
            sets.append(s(2, 5, kg(backoff), rest=120, label="Backoff"))
        ex(name, weeks=[week], desc=desc if week == 1 else None, warmup=warmup, sets=sets)

def conditioning(n, name):
    day(1, n, name, desc="Easy pace, nasal breathing. Recovery, not a workout.", kind="conditioning", weeks="1-7")
    ex("Jump Rope", desc="Ten rounds: one minute on, thirty seconds off.",
       sets=[s(10, 1, bw(), work=60, rest=30, auto=True)])
    ex("Squat, Bodyweight", superset="circuit", desc="Circuit: move to the next exercise without resting.",
       sets=[s(3, 20, bw())])
    ex("Reverse Lunge", alts=["Split Squat"], superset="circuit", desc="Alternate legs; 10 is per side.",
       sets=[s(3, 10, bw())])
    ex("Glute Bridge", alts=["Hip Thrust, Bodyweight", "Single Leg Bridge"], superset="circuit",
       desc="Squeeze for a second at the top.", sets=[s(3, 15, bw())])
    ex("Step up", alts=["Squat, Bodyweight"], superset="circuit", desc="Knee-height box, drive through the heel.",
       sets=[s(3, 10, bw())])
    ex("Dead Bug", alts=["Plank"], superset="circuit", desc="Lower back stays flat on the floor.",
       sets=[s(3, 10, bw())])
    ex("Plank", desc="Hold the full 45 seconds, then rest.", sets=[s(3, 1, bw(), work=45, rest=60)])

# Shared by both upper days, so the two sessions progress one exercise.
# Light accessories name their equipment, so they round to dumbbells or a
# cable stack, not to the 20 kg bar, and skip the default warmup.
LATERAL_RAISE = template(eq="Dumbbell", warmup=[], sets=[s(3, 12, kg(12.5))], progress=dp(kg(2.5), 12, 15))

def shoulder_health():
    ex("Reverse Fly, Band", alts=["Reverse Fly"], desc="Light and strict; this is for the rotator cuff.",
       sets=[s(3, 20, kg(0), rest=30)])
    ex("Face Pull", desc="Pull to the forehead, thumbs back.", sets=[s(3, 15, kg(20), rest=45)],
       progress=dp(kg(2.5), 15, 15))
    ex("Side Lying External Rotation", eq="Dumbbell", warmup=[], sets=[s(2, 15, kg(3), rest=30)])

conditioning(1, "Mon conditioning")

day(1, 2, "Tue Upper A bench", desc="Bench day. Work sets stop one or two reps short of failure.", weeks="1-7")
main_lift("Bench Press", BENCH, BAR_WARMUP, "Pause each rep on the chest. Backoffs keep the same form.")
ex("Lat Pulldown", alts=["Pull Up"], label="heavy", desc="Full stretch at the top, chest to the bar.",
   warmup=[w(1, 8, kg(40))], sets=[s(4, 6, kg(70), rest=90)], progress=dp(kg(2.5), 6, 8))
ex("Overhead Press", alts=["Shoulder Press Parallel Grip"], desc="Glutes tight, no lean back.",
   sets=[s(3, 6, kg(42.5))], progress=dp(kg(2.5), 6, 8))
ex("Incline Row", alts=["T Bar Row"], sets=[s(3, 10, kg(20))], progress=dp(kg(2.5), 10, 12))
ex("Lateral Raise", alts=["Lateral Raise, Cable"], desc="Lead with the elbows.", **LATERAL_RAISE)
ex("Bicep Curl, Cable", alts=["Bicep Curl, EZ Bar"], warmup=[], sets=[s(3, 10, kg(10))], progress=dp(kg(2.5), 10, 12))
ex("Triceps Extension", alts=["Triceps Extension, Cable", "Skullcrusher"], sets=[s(3, 12, kg(20))],
   progress=dp(kg(2.5), 12, 12))
ex("Reverse Curl", alts=["Reverse Curl, Cable", "Wrist Roller"], eq="Dumbbell", warmup=[], desc="Forearms and elbows.",
   sets=[s(2, 12, kg(8))], progress=dp(kg(2.5), 12, 12))
shoulder_health()

day(1, 3, "Wed Lower B deadlift", desc="Deadlift day. Reset each rep on the floor.", weeks="1-7")
main_lift("Deadlift", DEADLIFT, DEADLIFT_WARMUP, "Wedge in, take the slack out, push the floor away.")
ex("Hack Squat", alts=["Front Squat"], sets=[s(3, 8, kg(40))], progress=dp(kg(5), 8, 8))
ex("Bulgarian Split Squat", alts=["Reverse Lunge, Bodyweight", "Lunge, Dumbbell"], eq="Dumbbell", warmup=[],
   desc="8 per leg.", sets=[s(3, 8, kg(12))], progress=dp(kg(2.5), 8, 8))
ex("Back Extension", alts=["Good Morning", "Reverse Hyperextension"], sets=[s(3, 12, kg(0))],
   progress=dp(kg(5), 12, 12))
ex("Seated Calf Raise", alts=["Standing Calf Raise, Bodyweight"], desc="Pause in the stretch.",
   sets=[s(3, 15, kg(40))], progress=dp(kg(5), 15, 15))
ex("Ab Wheel", alts=["Crunch, Cable", "Hollow Body Hold"], sets=[s(3, 10, kg(0))])
ex("Dead Hang", desc="Hang past 30 seconds if you can; the timer counts up.",
   sets=[s(3, 1, kg(0), work=30, count_up=True, rest=60)])

conditioning(4, "Thu conditioning")

day(1, 5, "Fri Upper B volume", desc="Volume day. Moderate loads, controlled tempo.", weeks="1-7")
ex("Chest Press, Leverage Machine", alts=["Bench Press, Dumbbell"], sets=[s(4, 8, kg(60))],
   progress=dp(kg(5), 8, 12))
ex("Bench Press Close Grip", alts=["Bench Press Close Grip, Smith Machine"], desc="Elbows tucked.",
   sets=[s(3, 6, kg(47.5))], progress=dp(kg(2.5), 6, 8))
ex("Pendlay Row", alts=["Bent Over Row, Dumbbell"], desc="Bar back to the floor each rep.",
   sets=[s(4, 6, kg(60))], progress=dp(kg(2.5), 6, 8))
ex("Lat Pulldown", alts=["Seated Row"], label="volume", sets=[s(3, 10, kg(50))], progress=dp(kg(2.5), 10, 12))
ex("Lateral Raise", **LATERAL_RAISE)
ex("Preacher Curl", alts=["Preacher Curl, Leverage Machine", "Concentration Curl"], sets=[s(3, 8, kg(20))],
   progress=dp(kg(2.5), 8, 10))
ex("Hammer Curl", alts=["Hammer Curl, Cable"], eq="Dumbbell", warmup=[], sets=[s(3, 10, kg(12))],
   progress=dp(kg(2.5), 10, 12))
ex("Triceps Pushdown", alts=["Triceps Extension, Band"], sets=[s(3, 12, kg(25))], progress=dp(kg(2.5), 12, 12))
ex("Wrist Curl", alts=["Wrist Curl, Dumbbell"], eq="Dumbbell", warmup=[], sets=[s(2, 15, kg(15))],
   progress=dp(kg(2.5), 15, 15))
ex("Reverse Wrist Curl", alts=["Reverse Wrist Curl, Dumbbell"], eq="Dumbbell", warmup=[], sets=[s(2, 15, kg(10))],
   progress=dp(kg(2.5), 15, 15))
shoulder_health()

day(1, 6, "Sat Lower A squat", desc="Squat day. Same depth on every rep.", weeks="1-7")
main_lift("Squat", SQUAT, BAR_WARMUP, "Hip crease below the knee; brace before each rep.")
ex("Romanian Deadlift", alts=["Stiff Leg Deadlift, Dumbbell", "Cable Pull Through"],
   desc="Hinge until the hamstrings stop you.", sets=[s(3, 8, kg(90))], progress=lp(kg(2.5)))
ex("Leg Press", alts=["Hack Squat", "Goblet Squat"], sets=[s(3, 10, kg(120))], progress=dp(kg(5), 10, 10))
ex("Seated Leg Curl", alts=["Lying Leg Curl", "Nordic Curl, Band"], sets=[s(3, 12, kg(60))],
   progress=dp(kg(2.5), 12, 12))
ex("Calf Press on Leg Press", alts=["Standing Calf Raise, Leverage Machine", "Standing Calf Raise, Bodyweight"],
   sets=[s(3, 15, kg(80))], progress=dp(kg(5), 15, 15))
ex("Hanging Leg Raise", alts=["Hanging Knee Raise", "Flat Leg Raise"], desc="No swinging.", sets=[s(3, 12, kg(0))])

conditioning(7, "Sun conditioning")

# Week 8: accessories and conditioning stop (they run weeks 1-7).
day(8, 1, "Mon walk only", desc="A 30-45 minute easy walk. No lifting.", kind="note")
day(8, 2, "Tue test singles", desc="Two crisp singles per lift, nothing grinding. Squat, bench, then deadlift.")
ex("Squat", warmup=BAR_WARMUP, sets=[s(2, 1, kg(100), rest=180, label="Work")])
ex("Bench Press", warmup=BAR_WARMUP, sets=[s(2, 1, kg(82.5), rest=180, label="Work")])
ex("Deadlift", warmup=DEADLIFT_WARMUP, sets=[s(2, 1, kg(130), rest=180, label="Work")])
