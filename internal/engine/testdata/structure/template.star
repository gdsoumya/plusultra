program("P", weeks=1)

def bump(ctx):
    pass

t = template(sets=[s(3, 10, kg(20))], warmup=[], progress=custom(bump, attempt=0))
day(1, 1, "Accessories")
ex("Curl", **t)
ex("Lateral Raise", label="b", **t)
