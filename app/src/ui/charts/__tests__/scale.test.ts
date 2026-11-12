import { linear, linePath, niceTicks } from "../scale";

describe("linear", () => {
  it("maps the domain onto the range", () => {
    const scale = linear([0, 10], [100, 0]);
    expect(scale(0)).toBe(100);
    expect(scale(5)).toBe(50);
    expect(scale(10)).toBe(0);
  });

  it("puts a flat domain in the middle of the range", () => {
    expect(linear([3, 3], [0, 100])(3)).toBe(50);
  });
});

describe("niceTicks", () => {
  it("covers the values with round steps", () => {
    expect(niceTicks(82.7, 84.8)).toEqual([82, 83, 84, 85]);
    expect(niceTicks(95, 105)).toEqual([95, 100, 105]);
    expect(niceTicks(3, 10)).toEqual([2, 4, 6, 8, 10]);
  });

  it("keeps decimals exact", () => {
    expect(niceTicks(0.1, 0.9)).toEqual([0, 0.2, 0.4, 0.6, 0.8, 1]);
  });

  it("widens a single value", () => {
    expect(niceTicks(100, 100)).toEqual([99, 99.5, 100, 100.5, 101]);
  });
});

describe("linePath", () => {
  it("moves to the first point and draws lines to the rest", () => {
    expect(
      linePath([
        { x: 0, y: 10.26 },
        { x: 5, y: 7 },
      ]),
    ).toBe("M0 10.3 L5 7");
  });
});
