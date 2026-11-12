meal_program("Fallback", weeks=6)
day(1, kind="rest", target=target(kcal=2000))
day(3, kind="rest", weeks="3-4", target=target(kcal=1900))
day(2, kind="training", target=target(kcal=2500))
