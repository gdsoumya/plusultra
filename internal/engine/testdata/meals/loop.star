meal_program("Six week cut", weeks=6)
chicken = food("Chicken", per100g={"kcal": 120, "protein": 23})
plan = {"training": 2600, "rest": 2200}
for w in range(1, 7):
    step = 250 * ((w - 1) // 3)
    for kind, kcal in plan.items():
        day(w, kind=kind, target=target(kcal=kcal - step))
        meal("Lunch", chicken(200))
