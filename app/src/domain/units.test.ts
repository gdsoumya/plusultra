import { formatWeight, fromKg, toKg } from "./units";

describe("units", () => {
  it("formats a stored weight in the user's unit", () => {
    expect(formatWeight({ value: 100, unit: "kg" }, "kg")).toBe("100 kg");
    expect(formatWeight({ value: 62.5, unit: "kg" }, "kg")).toBe("62.5 kg");
    expect(formatWeight({ value: 100, unit: "kg" }, "lb")).toBe("220.5 lb");
    expect(formatWeight({ value: 225, unit: "lb" }, "kg")).toBe("102.1 kg");
  });

  it("rounds a weight the server already sent in the user's unit", () => {
    // 110 kg resolved for an lb user arrives as 242.50848840336533 lb.
    expect(formatWeight({ value: 242.50848840336533, unit: "lb" }, "lb")).toBe("242.5 lb");
    expect(formatWeight({ value: 101.25, unit: "kg" }, "kg")).toBe("101.25 kg");
  });

  it("converts kilograms to and from the user's unit", () => {
    expect(fromKg(100, "kg")).toBe(100);
    expect(fromKg(100, "lb")).toBe(220.5);
    expect(toKg(100, "kg")).toBe(100);
    expect(toKg(225, "lb")).toBe(102.06);
  });
});
