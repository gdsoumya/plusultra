import { create } from "@bufbuild/protobuf";

import { RoundingSchema } from "../../gen/plusultra/v1/common_pb";
import { describeRounding, draftOf, parseRounding } from "./rounding";

const plates = create(RoundingSchema, { kind: "plates", bar: 20, plates: [25, 20, 10, 5, 2.5] });

describe("rounding", () => {
  it("round-trips a rounding through its editable draft", () => {
    const draft = draftOf(plates);
    expect(draft).toEqual({
      kind: "plates",
      bar: "20",
      increment: "",
      plates: "25, 20, 10, 5, 2.5",
    });
    expect(parseRounding(draft)).toEqual(plates);
  });

  it("parses each kind and keeps only its fields", () => {
    expect(parseRounding({ kind: "stack", bar: "20", increment: "2.5", plates: "" })).toEqual(
      create(RoundingSchema, { kind: "stack", increment: 2.5 }),
    );
    expect(parseRounding({ kind: "fixed", bar: "", increment: "", plates: "8 12 16" })).toEqual(
      create(RoundingSchema, { kind: "fixed", plates: [8, 12, 16] }),
    );
    expect(parseRounding({ kind: "bodyweight", bar: "x", increment: "", plates: "" })).toEqual(
      create(RoundingSchema, { kind: "bodyweight" }),
    );
  });

  it("rejects numbers it cannot read", () => {
    expect(() =>
      parseRounding({ kind: "plates", bar: "abc", increment: "", plates: "20" }),
    ).toThrow("Bar must be a number");
    expect(() =>
      parseRounding({ kind: "plates", bar: "20", increment: "", plates: "20, x" }),
    ).toThrow("Plates must be numbers above 0");
    expect(() => parseRounding({ kind: "stack", bar: "", increment: "0", plates: "" })).toThrow(
      "Increment must be above 0",
    );
  });

  it("describes a rounding in one line", () => {
    expect(describeRounding(plates)).toBe("Bar 20, plates 25/20/10/5/2.5");
    expect(describeRounding(create(RoundingSchema, { kind: "fixed", plates: [8, 12, 16] }))).toBe(
      "3 weights, 8 to 16",
    );
    expect(describeRounding(create(RoundingSchema, { kind: "stack", increment: 5 }))).toBe(
      "Steps of 5",
    );
    expect(describeRounding(create(RoundingSchema, { kind: "bodyweight" }))).toBe("No rounding");
  });
});
