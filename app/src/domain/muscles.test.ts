import { readFileSync } from "fs";
import { join } from "path";

import { groupOf, muscleGroups, muscleRegions } from "./muscles";

// The server file is the source of truth; this reads its Groups and Regions literals.
const goSource = readFileSync(join(__dirname, "../../../internal/catalog/muscles.go"), "utf8");
const between = (from: string, to: string) =>
  goSource.slice(goSource.indexOf(from), goSource.indexOf(to));

describe("muscle taxonomy", () => {
  it("lists the server's groups in the same order", () => {
    const groups = [...between("var Groups", "var Regions").matchAll(/"([^"]+)"/g)].map(
      (m) => m[1],
    );
    expect(muscleGroups).toEqual(groups);
  });

  it("maps every region to the same group as the server", () => {
    const pairs = [...between("var Regions", "var synonyms").matchAll(/"([^"]+)":\s*"([^"]+)"/g)];
    const server = Object.fromEntries(pairs.map((m) => [m[1], m[2]]));
    const app = Object.fromEntries(
      muscleGroups.flatMap((g) => muscleRegions[g].map((r) => [r, g])),
    );
    expect(app).toEqual(server);
  });

  it("rolls a region up to its group and leaves other names alone", () => {
    expect(groupOf("front delts")).toBe("shoulders");
    expect(groupOf("chest")).toBe("chest");
    expect(groupOf("unknown")).toBe("unknown");
  });
});
