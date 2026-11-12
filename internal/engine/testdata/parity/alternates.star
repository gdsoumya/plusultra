# Reference syntax: Squat, Bodyweight | ! Pistol Squat | Front Squat / 3x8 0kg
#              Pull Up | Chin Up / 3x5 0kg
program("Alternates with current marker", weeks=2)
day(1, 1, "Lower", weeks="1-2")
ex(alts=["Squat, Bodyweight", "Pistol Squat", "Front Squat"], current=2, sets=[s(3, 8, bw())])
ex("Pull Up", alts=["Chin Up"], sets=[s(3, 5, bw())])
