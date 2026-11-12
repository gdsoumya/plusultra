program("P", weeks=8)
day(1, 1, "Lower")
ex("Squat", weeks=range(1, 8), sets=[s(3, 5, kg(100))])
ex("Plank", sets=[s(3, 1, bw(), work=60)])
