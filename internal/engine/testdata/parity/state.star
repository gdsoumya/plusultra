# Reference syntax: Bench Press / 3x8 80kg / progress: custom(sessions: 0, rating+: 3) {~
#                state.sessions += 1
#                if (state.rating >= 4) { weights += 2.5kg }
#              ~}
# rating is prompted after the exercise; the parity harness answers 4.
program("State and prompted state", weeks=2)

def rate(ctx):
    ctx.state.sessions += 1
    ctx.state.history.append(ctx.completed_reps[1])
    ctx.state.best = max(ctx.state.best, ctx.completed_weights[1])
    if ctx.state.rating >= 4:
        ctx.weights += kg(2.5)

day(1, 1, "Upper", weeks="1-2")
ex("Bench Press", warmup=[], sets=[s(3, 8, kg(80))],
   progress=custom(rate, sessions=0, rating=prompt(3), history=[]), state={"best": kg(0)})
