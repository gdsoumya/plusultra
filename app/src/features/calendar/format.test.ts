import { kcal } from "./format";

it("rounds calories", () => {
  expect(kcal(2599.6)).toBe("2600 kcal");
});
