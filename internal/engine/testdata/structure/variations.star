program("P", weeks=1)
day(1, 1, "Lower")
ex("Squat", variations=[
    [s(3, 5, kg(100))],
    [s(5, 3, kg(110)), s(1, 5, kg(90), amrap=True)],
])
