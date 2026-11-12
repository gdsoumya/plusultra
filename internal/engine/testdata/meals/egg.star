meal_program("Eggs", weeks=1)
egg = food("Egg", per100g={"kcal": 143}, per_unit={"kcal": 70, "protein": 6}, unit="egg")
day(1, 1)
meal("Breakfast", item(egg, 3), egg(50, unit="g"))
