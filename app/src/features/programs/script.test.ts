import { insertExercise, replaceName } from "./script";

const script = 'program("X", weeks=1)\nday(1, 1, "A")\nex(, sets=[s(3, 5, kg(60))])\n';
const inParens = script.indexOf("ex(") + 3;

describe("insertExercise", () => {
  it("inserts the quoted name mid-line and puts the cursor after it", () => {
    const r = insertExercise(script, { start: inParens, end: inParens }, "Squat");
    expect(r.script).toContain('ex("Squat", sets=[s(3, 5, kg(60))])');
    expect(r.cursor).toBe(inParens + '"Squat"'.length);
  });

  it("inserts a whole ex line at a line start", () => {
    const r = insertExercise(script, { start: script.length, end: script.length }, "Squat");
    expect(r.script.endsWith('\nex("Squat", sets=[s(3, 5, kg(60))])')).toBe(true);
    expect(r.cursor).toBe(r.script.length);
  });

  it("breaks the line when text follows the insertion point", () => {
    const start = script.indexOf("day(");
    const r = insertExercise(script, { start, end: start }, "Squat");
    expect(r.script.split("\n")[1]).toBe('ex("Squat", sets=[s(3, 5, kg(60))])');
    expect(r.script.split("\n")[2]).toBe('day(1, 1, "A")');
  });

  it("replaces a selection", () => {
    const start = script.indexOf('"X"');
    const r = insertExercise(script, { start, end: start + 3 }, "Squat");
    expect(r.script.startsWith('program("Squat", weeks=1)')).toBe(true);
  });
});

describe("replaceName", () => {
  it("swaps the quoted name on the line only", () => {
    const bad = 'ex("Sqat")\nex("Sqat")\n';
    expect(replaceName(bad, 2, "Sqat", "Squat")).toBe('ex("Sqat")\nex("Squat")\n');
  });

  it("leaves the script alone for a line past the end", () => {
    expect(replaceName("a\n", 5, "a", "b")).toBe("a\n");
  });
});
