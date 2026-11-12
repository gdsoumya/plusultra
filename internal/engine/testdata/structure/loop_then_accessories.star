program("P", weeks=4)
for w in range(1, 5):
    day(w, 1, "Lower", desc="Deload" if w == 4 else "")
    ex("Squat", sets=[s(3, 5, kg(100 + 5 * w))])
day(1, 1, "Lower", weeks="1-4")
ex("Leg Curl", sets=[s(3, 12, kg(30))])
