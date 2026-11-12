# Reference syntax: Overhead Press / 3x8 40kg / progress: custom() {~
#                state[1].rating = 10
#                state[101].rating = 5
#              ~}
#              Squat / 3x8 100kg / id: tags(1) / progress: custom(rating: 0) {~ if (state.rating >= 8) { weights += 5kg } ~}
#              Bench Press / 3x8 70kg / id: tags(1, 101) / progress: custom(rating: 0) {~ print(state.rating) ~}
# Shared state lives under the tag, and any exercise reaches it, tagged or not.
program("Tags and shared state", weeks=2)

def give(ctx):
    ctx.shared(1).rating = 10
    ctx.shared(101).rating = 5

def heavy_if_rated(ctx):
    if getattr(ctx.shared(1), "rating", 0) >= 8:
        ctx.weights += kg(5)

def show(ctx):
    print(getattr(ctx.shared(1), "rating", 0), getattr(ctx.shared(101), "rating", 0))

day(1, 1, "Full body", weeks="1-2")
ex("Overhead Press", warmup=[], sets=[s(3, 8, kg(40))], progress=custom(give))
ex("Squat", tags=[1], warmup=[], sets=[s(3, 8, kg(100))], progress=custom(heavy_if_rated))
ex("Bench Press", tags=[1, 101], warmup=[], sets=[s(3, 8, kg(70))], progress=custom(show))
