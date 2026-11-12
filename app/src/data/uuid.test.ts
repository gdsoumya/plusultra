import { insecureOrigin } from "./test-utils";
import { uuid } from "./uuid";

// What expo-crypto's web module does: hand both calls to the page's crypto.
jest.mock("expo-crypto", () => ({
  randomUUID: () => globalThis.crypto.randomUUID(),
  getRandomValues: (a: Uint8Array<ArrayBuffer>) => globalThis.crypto.getRandomValues(a),
}));

it("makes distinct v4 UUIDs on a web page served over plain http", () => {
  insecureOrigin();
  const ids = Array.from({ length: 100 }, uuid);
  for (const id of ids) {
    expect(id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  }
  expect(new Set(ids).size).toBe(ids.length);
});
