# Recomp meals for the 16-week strength plan, weeks 1-8.
#
# Kind days follow the workout calendar: training days eat the most,
# conditioning days less, rest days least. Targets rise with the strength
# block in week 5. In week 8, numbered days cover Monday's walk (a note day in
# the workout plan) and Tuesday's test singles. Macros here are approximate;
# real programs can take them from the food catalog with ref=.
meal_program("Recomp meals", weeks=8, loop=False, notes="Pairs with the 16-week strength plan")

oats = food("Oats", per100g={"kcal": 379, "protein": 13.2, "carbs": 67.7, "fat": 6.5, "fiber": 10.1})
whey = food("Whey protein", per100g={"kcal": 400, "protein": 80, "carbs": 8, "fat": 6})
egg = food("Egg, whole", per100g={"kcal": 143, "protein": 12.6, "carbs": 0.7, "fat": 9.5},
           per_unit={"kcal": 72, "protein": 6.3, "carbs": 0.4, "fat": 4.8}, unit="egg")
chicken = food("Chicken breast, raw", per100g={"kcal": 120, "protein": 22.5, "fat": 2.6})
rice = food("Basmati rice, raw", per100g={"kcal": 356, "protein": 8.1, "carbs": 77.7, "fat": 0.6, "fiber": 1.3})
lentils = food("Red lentils, raw", per100g={"kcal": 358, "protein": 24, "carbs": 60, "fat": 2.2, "fiber": 11})
yogurt = food("Greek yogurt, nonfat", per100g={"kcal": 59, "protein": 10.3, "carbs": 3.6, "fat": 0.4})
banana = food("Banana", per100g={"kcal": 89, "protein": 1.1, "carbs": 22.8, "fat": 0.3, "fiber": 2.6})
oil = food("Olive oil", per100g={"kcal": 884, "fat": 100})

def breakfast():
    meal("Breakfast", oats(80), whey(30), banana(120), time="07:30")

def dinner(rice_g):
    meal("Dinner", chicken(200), rice(rice_g), oil(10), time="20:00")

def block(week, weeks, kcal):
    """Training, conditioning and rest days for the listed weeks."""
    day(week, kind="training", name="Training day", weeks=weeks,
        target=target(kcal=kcal, protein=180, water_ml=3500))
    breakfast()
    meal("Lunch", egg(3), lentils(80), rice(60), time="13:00")
    meal("After training", yogurt(250), banana(120), desc="Within an hour of the session.")
    dinner(100)

    day(week, kind="conditioning", name="Conditioning day", weeks=weeks, target=target(kcal=kcal - 200, protein=180))
    breakfast()
    meal("Lunch", egg(3), lentils(80), time="13:00")
    dinner(80)

    day(week, kind="rest", name="Rest day", weeks=weeks, target=target(kcal=kcal - 400, protein=170))
    breakfast()
    meal("Lunch", egg(3), lentils(60), time="13:00")
    dinner(50)

block(1, "1-4", 2600)
block(5, "5-8", 2800)

day(8, 1, "Walk day", target=target(kcal=2300, protein=170))
breakfast()
meal("Lunch", egg(3), lentils(60), time="13:00")
dinner(50)

day(8, 2, "Test day", target=target(kcal=3000, protein=180))
breakfast()
meal("Before testing", rice(80), banana(120), time="11:00")
dinner(120)
