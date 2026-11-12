meal_program("Maximal", weeks=1)
m = {k: 100000 for k in ["kcal", "protein", "carbs", "fat", "fiber", "water_ml"]}
big = food("Big", per100g=m, per_unit=m, unit="unit")
day(1, 1)
meal("All", *[big(100000) for _ in range(1000)])
