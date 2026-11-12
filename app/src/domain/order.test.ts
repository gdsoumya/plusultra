import { movedIndex, moveItem } from "./order";

describe("movedIndex", () => {
  it("follows an item through a move of it or of another item", () => {
    expect(movedIndex(0, 0, 2)).toBe(2);
    expect(movedIndex(1, 0, 2)).toBe(0);
    expect(movedIndex(2, 0, 2)).toBe(1);
    expect(movedIndex(3, 0, 2)).toBe(3);
    expect(movedIndex(1, 3, 1)).toBe(2);
    expect(movedIndex(3, 3, 1)).toBe(1);
    expect(movedIndex(0, 3, 1)).toBe(0);
  });
});

describe("moveItem", () => {
  const list = ["a", "b", "c", "d"];
  it("moves an item to a new index", () => {
    expect(moveItem(list, 0, 2)).toEqual(["b", "c", "a", "d"]);
    expect(moveItem(list, 3, 1)).toEqual(["a", "d", "b", "c"]);
  });
  it("clamps the target and returns the same list for a no-op", () => {
    expect(moveItem(list, 1, 9)).toEqual(["a", "c", "d", "b"]);
    expect(moveItem(list, 1, -3)).toEqual(["b", "a", "c", "d"]);
    expect(moveItem(list, 2, 2)).toBe(list);
  });
});
